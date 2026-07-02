package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"nunezlagos/domain/internal/audit"
	usvc "nunezlagos/domain/internal/service/issue"
	"nunezlagos/domain/internal/service/openspec"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	tsvc "nunezlagos/domain/internal/service/task"
	"nunezlagos/domain/internal/tracing"
)

// PhaseResultInput es lo que el cliente IDE reporta vía MCP cuando
// termina una fase. El service lo valida (D5 saves + handler.Validate)
// y persiste el resultado.
type PhaseResultInput struct {
	FlowRunStepID   uuid.UUID
	Output          map[string]any
	MemoryRefsSaved []phases.MemoryRef
	// ToolCallsSaved son los nombres de las tools domain_* que el cliente
	// invocó durante la fase (REQ-54 issue-54.1). El servidor valida que el
	// contrato required_tool_calls de la fase sea subconjunto de esta lista.
	ToolCallsSaved []string
	DurationMS     int64
}

// PhaseResultResult es lo que devolvemos al cliente: status del step,
// status agregado del flow_run (si terminaron todos los steps), y
// next_step opcional (slug + id del siguiente step pending si hay).
type PhaseResultResult struct {
	StepID         uuid.UUID
	StepStatus     string
	FlowRunStatus  string
	NextStepID     *uuid.UUID
	NextStepKey    string
	NextStepPrompt string





	RequiresConfirm bool
	ConfirmMessage  string

	// MissingToolCalls: si la fase declara un contrato required_tool_calls y el
	// cliente no reportó todas, el step NO se cierra (queda running, reintentable)
	// y acá van las tools que faltan para que el cliente sepa qué llamar (REQ-54).
	MissingToolCalls []string `json:"missing_tool_calls,omitempty"`

	SkillsRecommended *SkillsRecommended `json:"skills_recommended,omitempty"`




	MultiConcern *MultiConcernInfo `json:"multi_concern,omitempty"`




	Summary string `json:"summary,omitempty"`
}

// MultiConcernInfo modela la detección de multi-concern en sdd-explore
// (RFC 0006 D2). Contiene la lista de concerns separables que el cliente
// puede convertir en sub-flows independientes.
type MultiConcernInfo struct {
	Concerns []ConcernInfo `json:"concerns"`
}

// ConcernInfo describe un concern separable detectado por sdd-explore.
type ConcernInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RecordPhaseResult procesa el reporte del cliente sobre una fase:
//
//   1. Lookup del step + flow_run para sanity check
//   2. Validar contract D5 (suggested_saves required presentes)
//   3. Llamar handler.Validate del registry para chequeos shape-specific
//   4. Si todo verde → MarkStepCompleted; calcular si flow_run terminó
//   5. Si falla validación → MarkStepFailed con el error como mensaje
//
// Devuelve PhaseResultResult con el status final del step + flow + next
// step pending si aún hay fases por correr.
func (s *Service) RecordPhaseResult(ctx context.Context, in PhaseResultInput) (*PhaseResultResult, error) {
	ctx, span := tracing.Tracer("orchestrator").Start(ctx, "orchestrator.phase_result",
		trace.WithAttributes(
			tracing.SafeAttr("flow_run_step.id", in.FlowRunStepID.String()),
		),
	)
	defer span.End()

	if s.Repo == nil {
		err := errors.New("orchestrator: Repo not configured")
		span.RecordError(err)
		return nil, err
	}
	step, err := s.Repo.GetFlowRunStep(ctx, in.FlowRunStepID)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	span.SetAttributes(
		tracing.SafeAttr("phase.slug", step.StepKey),
		tracing.SafeAttr("flow_run.id", step.FlowRunID.String()),
	)
	if modeStr, _ := step.Inputs["mode"].(string); modeStr != "" {
		span.SetAttributes(tracing.SafeAttr("orchestrator.mode", modeStr))
	}
	if step.Status != "pending" && step.Status != "running" {
		return nil, ErrFlowRunStepNotPending
	}
	flowRun, err := s.Repo.GetFlowRun(ctx, step.FlowRunID)
	if err != nil {
		return nil, err
	}




	rebuilt := rebuildOutputFromStepInputs(step)
	phaseSlug := phases.PhaseSlug(step.StepKey)


	if err := ValidateRequiredSaves(phaseSlug, rebuilt,
		phases.ClientResult{Output: in.Output, MemoryRefsSaved: in.MemoryRefsSaved}); err != nil {




		_ = s.Repo.MarkStepFailed(ctx, step.ID, err.Error())
		_ = s.propagateFlowStatusAfterFailure(ctx, flowRun.ID)


		if s.Metrics != nil {
			var rse *RequiredSaveError
			if errors.As(err, &rse) {
				for _, m := range rse.Missing {
					s.Metrics.OrchestratorRequiredSaveMissingTotal.
						WithLabelValues(string(phaseSlug), m.Type).Inc()
				}
			}
			modeStr, _ := step.Inputs["mode"].(string)
			s.Metrics.OrchestratorPhaseResultsTotal.
				WithLabelValues(string(phaseSlug), modeStr, "failed").Inc()
		}
		return nil, err
	}


	if s.Phases != nil {
		if h, lookupErr := s.Phases.Lookup(phases.PhaseSlug(step.StepKey)); lookupErr == nil {
			result := phases.ClientResult{
				Output:          in.Output,
				MemoryRefsSaved: in.MemoryRefsSaved,
			}
			if err := h.Validate(ctx, rebuilt, result); err != nil {
				_ = s.Repo.MarkStepFailed(ctx, step.ID, err.Error())
				_ = s.propagateFlowStatusAfterFailure(ctx, flowRun.ID)
				return nil, err
			}
		}
	}

	// REQ-54 issue-54.1: contrato fase→tools. El contrato efectivo es el override
	// de agent_templates.metadata.required_tool_calls (si está y no vacío) o el
	// default del handler (rebuilt.RequiredToolCalls). Si el contrato no es
	// subconjunto de lo que el cliente reportó, NO cerramos el step: queda
	// running (reintentable), NO se marca failed, y devolvemos missing_tool_calls
	// para que el cliente sepa qué llamar. Contrato vacío = no-op (retrocompat).
	if missing := s.missingToolCalls(ctx, flowRun.OrganizationID, phaseSlug, rebuilt, in.ToolCallsSaved); len(missing) > 0 {
		if s.Metrics != nil {
			modeStr, _ := step.Inputs["mode"].(string)
			s.Metrics.OrchestratorPhaseResultsTotal.
				WithLabelValues(string(phaseSlug), modeStr, "tool_contract_unmet").Inc()
		}
		return &PhaseResultResult{
			StepID:           step.ID,
			StepStatus:       step.Status,
			MissingToolCalls: missing,
		}, nil
	}

	if err := s.Repo.MarkStepCompleted(ctx, step.ID, in.Output); err != nil {
		return nil, fmt.Errorf("mark completed: %w", err)
	}
	span.SetAttributes(tracing.SafeAttr("phase.result", "completed"))

	s.persistOpenspec(ctx, step, flowRun, in, span)

	if s.Metrics != nil {
		modeStr, _ := step.Inputs["mode"].(string)
		s.Metrics.OrchestratorPhaseResultsTotal.
			WithLabelValues(string(phaseSlug), modeStr, "completed").Inc()
		if in.DurationMS > 0 {
			s.Metrics.OrchestratorPhaseDuration.
				WithLabelValues(string(phaseSlug), modeStr).
				Observe(float64(in.DurationMS) / 1000.0)
		}
	}


	steps, err := s.Repo.ListFlowRunSteps(ctx, flowRun.ID)
	if err != nil {
		return nil, fmt.Errorf("list steps for status: %w", err)
	}
	out := &PhaseResultResult{
		StepID:        step.ID,
		StepStatus:    "completed",
		FlowRunStatus: flowRun.Status,
	}

	if summary, ok := in.Output["summary"].(string); ok {
		out.Summary = summary
	}
	out.FlowRunStatus, out.NextStepID, out.NextStepKey = aggregateFlowStatus(steps)
	if out.FlowRunStatus != flowRun.Status {
		if err := s.Repo.UpdateFlowRunStatus(ctx, flowRun.ID, out.FlowRunStatus); err != nil {
			return nil, fmt.Errorf("update flow_run status: %w", err)
		}


		if s.Metrics != nil && (out.FlowRunStatus == "completed" || out.FlowRunStatus == "failed") {
			modeStr, _ := step.Inputs["mode"].(string)
			s.Metrics.OrchestratorRunsTotal.WithLabelValues(modeStr, out.FlowRunStatus).Inc()
		}
	}




	if phaseSlug == "sdd-explore" {
		if isMulti, _ := in.Output["multi_concern"].(bool); isMulti {
			concerns := extractConcerns(in.Output)
			if len(concerns) > 0 {


				for i := range steps {
					if (steps[i].Status == "pending" || steps[i].Status == "running") && steps[i].ID != step.ID {
						_ = s.Repo.MarkStepCancelled(ctx, steps[i].ID)
						steps[i].Status = "cancelled"
					}
				}
				out.MultiConcern = &MultiConcernInfo{Concerns: concerns}

				out.FlowRunStatus, out.NextStepID, out.NextStepKey = aggregateFlowStatus(steps)
				if out.FlowRunStatus != flowRun.Status {
					_ = s.Repo.UpdateFlowRunStatus(ctx, flowRun.ID, out.FlowRunStatus)
				}
				span.SetAttributes(tracing.SafeAttr("phase.multi_concern", true))
			}
		}
	}






	if out.NextStepID != nil {
		nextStep := findStepByID(steps, *out.NextStepID)
		if nextStep != nil {
			cached, _ := nextStep.Inputs["user_prompt"].(string)
			if cached != "" {
				out.NextStepPrompt = cached
			} else {

				built, err := s.rebuildNextStepPrompt(ctx, nextStep, steps)
				if err != nil {
					return nil, fmt.Errorf("rebuild next step prompt: %w", err)
				}
				out.NextStepPrompt = built
			}



			if shouldRequireConfirm(step, in.Output) {
				if err := s.Repo.MarkStepBlocked(ctx, nextStep.ID,
					"D1 confirm required: change exceeds express threshold"); err != nil {
					return nil, fmt.Errorf("mark next blocked: %w", err)
				}
				out.RequiresConfirm = true
				out.ConfirmMessage = "Express detected change exceeds threshold; call domain_orchestrate_confirm to proceed"
				span.SetAttributes(tracing.SafeAttr("phase.requires_confirm", true))
			}



			if !out.RequiresConfirm && requiresPhaseGate(flowRun.ExecMode, nextStep.StepKey) {
				if err := s.Repo.MarkStepBlocked(ctx, nextStep.ID,
					"phase gate ("+flowRun.ExecMode+"): aprobación requerida para "+nextStep.StepKey); err != nil {
					return nil, fmt.Errorf("mark next blocked (gate): %w", err)
				}
				out.RequiresConfirm = true
				out.ConfirmMessage = "La fase '" + nextStep.StepKey + "' requiere aprobacion (modo " + flowRun.ExecMode + "). Revise el resultado y llame domain_orchestrate_confirm(confirmed=true) para continuar, o false para abortar."
				span.SetAttributes(tracing.SafeAttr("phase.requires_confirm", true))
			}




			if !out.RequiresConfirm && flowRun.Hardspec && step.StepKey == "sdd-spec" {
				if err := s.Repo.MarkStepBlocked(ctx, nextStep.ID,
					"hardspec: reiteración humana del spec requerida"); err != nil {
					return nil, fmt.Errorf("mark next blocked (hardspec): %w", err)
				}
				out.RequiresConfirm = true
				out.ConfirmMessage = "hardspec: revise y enriquezca el spec; si hace falta re-redactelo (re-ejecute sdd-spec). Confirme con domain_orchestrate_confirm(confirmed=true) para continuar. La confirmacion queda auditada."
				if s.Audit != nil {
					audit.RecordOrLog(ctx, s.Audit, audit.Event{
						ActorType:  audit.ActorSystem,
						Action:     "hardspec.review_required",
						EntityType: "flow_run",
						EntityID:   &flowRun.ID,
						NewValues:  map[string]any{"phase": "sdd-spec"},
					})
				}
				span.SetAttributes(tracing.SafeAttr("phase.requires_confirm", true))
			}



			threshold := readFloat(nextStep.Inputs["skill_threshold"], 0)
			if threshold > 0 {
				agentSlug, _ := nextStep.Inputs["agent_template_slug"].(string)
				recs, err := s.fetchRecommendedSkills(ctx, flowRun.OrganizationID, flowRun.ProjectID, agentSlug, threshold)
				if err != nil {

					span.RecordError(err)
				} else {
					out.SkillsRecommended = recs
				}
			}
		}
	}
	return out, nil
}

// shouldRequireConfirm evalúa D1: Express + apply completado + scope >
// threshold. Lee mode + express_max_lines del step.inputs (replicados al
// persistir el plan) y compara contra output.files_changed length.
//
// El cliente IDE puede pasar lines_changed (count) en el output si lo
// tiene; si no, sólo evaluamos files_changed > 1 (multi-file).
// hybridGatePhases son las fases donde una decisión humana cambia el rumbo;
// en modo "hybrid" el flujo pausa solo en estas.
var hybridGatePhases = map[string]bool{
	"sdd-spec": true, "sdd-design": true, "sdd-apply": true, "sdd-judge": true,
}

// requiresPhaseGate decide si la próxima fase debe pausar para aprobación
// humana según el modo de ejecución de la corrida.
func requiresPhaseGate(execMode, nextPhaseSlug string) bool {
	switch execMode {
	case "manual":
		return true
	case "hybrid":
		return hybridGatePhases[nextPhaseSlug]
	default: // "auto" o vacío
		return false
	}
}

func shouldRequireConfirm(step *FlowRunStepRow, output map[string]any) bool {
	if step.StepKey != "sdd-apply" {
		return false
	}
	mode, _ := step.Inputs["mode"].(string)
	if mode != "express" {
		return false
	}
	maxLines := readNumber(step.Inputs["express_max_lines"], 10)
	files, _ := output["files_changed"].([]any)
	if len(files) > 1 {
		return true
	}
	lines := readNumber(output["lines_changed"], 0)
	return lines > maxLines
}

// readNumber extrae un int de un map[string]any tolerando float64
// (json.Unmarshal default) y enteros nativos.
func readNumber(v any, defaultVal int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return defaultVal
}

// readFloat extrae un float64 de un map[string]any.
func readFloat(v any, defaultVal float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return defaultVal
}

// ConfirmContinue desbloquea el step blocked tras el confirm condicional
// D1. Si confirmed=true → MarkStepPending (cliente puede ejecutar);
// si confirmed=false → MarkStepFailed + flow_run pasa a failed.
func (s *Service) ConfirmContinue(ctx context.Context, flowRunID uuid.UUID, confirmed bool) (*PhaseResultResult, error) {
	if s.Repo == nil {
		return nil, errors.New("orchestrator: Repo not configured")
	}
	steps, err := s.Repo.ListFlowRunSteps(ctx, flowRunID)
	if err != nil {
		return nil, err
	}
	var blocked *FlowRunStepRow
	for i := range steps {
		if steps[i].Status == "blocked" {
			blocked = &steps[i]
			break
		}
	}
	if blocked == nil {
		return nil, errors.New("orchestrator: no blocked step found for this flow_run")
	}
	if !confirmed {

		if err := s.Repo.MarkStepFailed(ctx, blocked.ID, "user_rejected_confirm"); err != nil {
			return nil, fmt.Errorf("mark step failed: %w", err)
		}
		_ = s.propagateFlowStatusAfterFailure(ctx, flowRunID)
		if s.Metrics != nil {
			s.Metrics.OrchestratorConfirmsTotal.WithLabelValues("false").Inc()
		}
		return &PhaseResultResult{
			StepID:        blocked.ID,
			StepStatus:    "failed",
			FlowRunStatus: "failed",
		}, nil
	}

	if err := s.Repo.MarkStepPending(ctx, blocked.ID); err != nil {
		return nil, err
	}
	if s.Metrics != nil {
		s.Metrics.OrchestratorConfirmsTotal.WithLabelValues("true").Inc()
	}
	userPrompt, _ := blocked.Inputs["user_prompt"].(string)
	return &PhaseResultResult{
		StepID:         blocked.ID,
		StepStatus:     "pending",
		FlowRunStatus:  "running",
		NextStepID:     &blocked.ID,
		NextStepKey:    blocked.StepKey,
		NextStepPrompt: userPrompt,
	}, nil
}

// findStepByID es helper local — pgx no retorna las rows por id por default.
func findStepByID(steps []FlowRunStepRow, id uuid.UUID) *FlowRunStepRow {
	for i := range steps {
		if steps[i].ID == id {
			return &steps[i]
		}
	}
	return nil
}

// rebuildNextStepPrompt — lazy build de user_prompt para el próximo step
// en modo Full. Reúne los outputs de los steps completados como
// PriorOutputs, llama handler.Build, persiste el inputs actualizado.
//
// Esto es lo que hace Full "lazy": en BuildFullPlan sólo el primer step
// recibe prompt; los demás quedan con UserPrompt="" hasta que su
// predecesor termine y disparemos esta función vía RecordPhaseResult.
func (s *Service) rebuildNextStepPrompt(ctx context.Context, next *FlowRunStepRow, allSteps []FlowRunStepRow) (string, error) {
	if s.Phases == nil {
		return "", nil
	}
	handler, err := s.Phases.Lookup(phases.PhaseSlug(next.StepKey))
	if err != nil {



		return "", nil
	}
	priorOutputs := collectPriorOutputs(allSteps, next.StepKey)
	rawText := extractRawTextFromInputs(allSteps, next)
	out, err := handler.Build(ctx, phases.Input{
		OrganizationID: uuid.Nil, // no necesario para Build (sólo composición de prompt)
		UserID:         uuid.Nil,
		FlowRunID:      next.FlowRunID,
		PhaseSlug:      phases.PhaseSlug(next.StepKey),
		RawText:        rawText,
		PriorOutputs:   priorOutputs,
	})
	if err != nil {
		return "", fmt.Errorf("handler.Build %s: %w", next.StepKey, err)
	}


	updatedInputs := mapClone(next.Inputs)
	updatedInputs["user_prompt"] = out.UserPrompt
	if err := s.Repo.UpdateStepInputs(ctx, next.ID, updatedInputs); err != nil {
		return "", err
	}
	return out.UserPrompt, nil
}

// collectPriorOutputs arma el map slug→output de todos los steps
// completed/skipped antes del próximo step. Sólo cuentan las fases
// efectivamente ejecutadas con éxito (failed steps no aportan output útil).
func collectPriorOutputs(steps []FlowRunStepRow, nextSlug string) map[phases.PhaseSlug]map[string]any {
	out := make(map[phases.PhaseSlug]map[string]any)
	for _, st := range steps {
		if st.StepKey == nextSlug {
			break
		}
		if st.Status != "completed" {
			continue
		}
		if len(st.Outputs) == 0 {
			continue
		}
		out[phases.PhaseSlug(st.StepKey)] = st.Outputs
	}
	return out
}

// extractRawTextFromInputs busca el raw_text original. Se persiste en
// flow_runs.cursor.raw_text pero como acceso rápido también en cada
// step.inputs (lo replicamos para evitar un join). Si no está, se
// busca en cualquier step.inputs.raw_text.
func extractRawTextFromInputs(steps []FlowRunStepRow, target *FlowRunStepRow) string {
	if target != nil {
		if rt, ok := target.Inputs["raw_text"].(string); ok && rt != "" {
			return rt
		}
	}
	for _, st := range steps {
		if rt, ok := st.Inputs["raw_text"].(string); ok && rt != "" {
			return rt
		}
	}
	return ""
}

// mapClone copia plana — necesario porque modificar el map del
// FlowRunStepRow leído de DB y luego pasarlo a UpdateStepInputs
// podría haber side-effects en tests si el caller retenía referencia.
func mapClone(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// propagateFlowStatusAfterFailure recalcula el status agregado y lo
// persiste tras marcar un step como failed. Mejor que repetir la
// lógica de aggregateFlowStatus inline en cada return-err.
func (s *Service) propagateFlowStatusAfterFailure(ctx context.Context, flowRunID uuid.UUID) error {
	steps, err := s.Repo.ListFlowRunSteps(ctx, flowRunID)
	if err != nil {
		return err
	}
	newStatus, _, _ := aggregateFlowStatus(steps)
	if err := s.Repo.UpdateFlowRunStatus(ctx, flowRunID, newStatus); err != nil {
		return err
	}



	if newStatus == "failed" {
		for _, st := range steps {
			if st.Status == "failed" && st.Error != "" {
				_ = s.Repo.SetFlowRunError(ctx, flowRunID, st.StepKey+": "+st.Error)
				break
			}
		}
	}
	return nil
}

// aggregateFlowStatus deriva el status del flow_run a partir de los
// steps + identifica el próximo step pending.
//
// Reglas:
//   - cualquier step failed                → flow failed
//   - todos los steps completed/skipped    → flow completed
//   - hay al menos uno pending/running     → flow running, next = primer pending
func aggregateFlowStatus(steps []FlowRunStepRow) (string, *uuid.UUID, string) {
	anyFailed := false
	allTerminal := true
	var nextID *uuid.UUID
	var nextKey string
	for i, st := range steps {
		switch st.Status {
		case "failed":
			anyFailed = true
		case "completed", "skipped", "cancelled":

		default:
			allTerminal = false
			if nextID == nil {
				id := steps[i].ID
				nextID = &id
				nextKey = st.StepKey
			}
		}
	}
	switch {
	case anyFailed:
		return "failed", nil, ""
	case allTerminal:
		return "completed", nil, ""
	default:
		return "running", nextID, nextKey
	}
}

// extractConcerns extrae la lista de concerns del output de sdd-explore.
// Espera el formato: {"multi_concern": true, "concerns": [{"name": "...", "description": "..."}]}
// Si el campo concerns no existe o no es un array válido, retorna nil.
func extractConcerns(output map[string]any) []ConcernInfo {
	raw, ok := output["concerns"].([]any)
	if !ok {
		return nil
	}
	var concerns []ConcernInfo
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		desc, _ := m["description"].(string)
		if name == "" {
			continue
		}
		concerns = append(concerns, ConcernInfo{Name: name, Description: desc})
	}
	return concerns
}

// persistOpenspec materializa el output de la fase en las tablas openspec
// (sdd_proposals, sdd_designs, issue_tasks). Best-effort: si falla registra
// error en span pero nunca interrumpe el flujo del orchestrator.
func (s *Service) persistOpenspec(ctx context.Context, step *FlowRunStepRow, flowRun *FlowRunRow, in PhaseResultInput, span trace.Span) {
	if s.IssueSvc == nil || (s.Spec == nil && s.Tasks == nil) {
		return
	}

	issueSlug, _ := in.Output["issue_slug"].(string)
	if issueSlug == "" {
		steps, err := s.Repo.ListFlowRunSteps(ctx, flowRun.ID)
		if err != nil {
			span.RecordError(err)
			return
		}
		issueSlug = findIssueSlug(steps)
	}

	if issueSlug == "" {
		return
	}

	issue, err := s.IssueSvc.GetBySlug(ctx, issueSlug)
	if err != nil {
		span.RecordError(fmt.Errorf("persistOpenspec: get issue %s: %w", issueSlug, err))
		return
	}

	switch step.StepKey {
	case "sdd-propose":
		s.persistProposalOpenspec(ctx, issue, in, span)
	case "sdd-design":
		s.persistDesignOpenspec(ctx, issue, in, span)
	case "sdd-tasks":
		s.persistTasksOpenspec(ctx, issue, in, span)
	}
}

func findIssueSlug(steps []FlowRunStepRow) string {
	for _, st := range steps {
		if st.Status != "completed" {
			continue
		}
		if slug, ok := st.Outputs["issue_slug"].(string); ok && slug != "" {
			return slug
		}
	}
	return ""
}

func (s *Service) persistProposalOpenspec(ctx context.Context, issue *usvc.Issue, in PhaseResultInput, span trace.Span) {
	if s.Spec == nil {
		return
	}
	proposalMD, ok := in.Output["proposal_md"].(string)
	if !ok || proposalMD == "" {
		return
	}
	doc := openspec.ParseProposal(proposalMD)
	_, err := s.Spec.CreateProposal(ctx, issue.ID, doc.Intention, doc.Scope, doc.Approach, doc.Risks, doc.TestingNotes)
	if err != nil {
		span.RecordError(fmt.Errorf("persistOpenspec: create proposal for %s: %w", issue.Slug, err))
	}
}

func (s *Service) persistDesignOpenspec(ctx context.Context, issue *usvc.Issue, in PhaseResultInput, span trace.Span) {
	if s.Spec == nil {
		return
	}
	designMD, ok := in.Output["design_md"].(string)
	if !ok || designMD == "" {
		return
	}
	doc := openspec.ParseDesign(designMD)

	var proposalID *uuid.UUID
	prop, err := s.Spec.GetLatestProposal(ctx, issue.ID)
	if err == nil {
		proposalID = &prop.ID
	}

	_, err = s.Spec.CreateDesign(ctx, issue.ID, proposalID, doc.ArchDecisions, doc.Alternatives, doc.DataFlow, doc.TDDPlan, doc.RisksMitigation)
	if err != nil {
		span.RecordError(fmt.Errorf("persistOpenspec: create design for %s: %w", issue.Slug, err))
	}
}

func (s *Service) persistTasksOpenspec(ctx context.Context, issue *usvc.Issue, in PhaseResultInput, span trace.Span) {
	if s.Tasks == nil {
		return
	}
	tasksRaw, ok := in.Output["tasks"].([]any)
	if !ok || len(tasksRaw) == 0 {
		return
	}

	inputs := make([]tsvc.CreateTaskInput, 0, len(tasksRaw))
	for _, raw := range tasksRaw {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		desc, _ := m["description"].(string)
		if desc == "" {
			continue
		}
		section, _ := m["section"].(string)
		if section == "" {
			section = "general"
		}
		inputs = append(inputs, tsvc.CreateTaskInput{Section: section, Description: desc})
	}

	if len(inputs) == 0 {
		return
	}

	_, err := s.Tasks.CreateTasks(ctx, issue.ID, inputs)
	if err != nil {
		span.RecordError(fmt.Errorf("persistOpenspec: create tasks for %s: %w", issue.Slug, err))
	}
}

// rebuildOutputFromStepInputs reconstruye un phases.Output desde el
// JSONB persistido en flow_run_steps.inputs. Sólo necesitamos los
// suggested_saves para D5 validation; el system/user prompt no se
// re-valida (eso ya pasó en handler.Build).
func rebuildOutputFromStepInputs(step *FlowRunStepRow) *phases.Output {
	out := &phases.Output{}
	// REQ-54 issue-54.1: reconstruir el default de required_tool_calls que el
	// handler declaró al planificar la fase (persistido en los Inputs del step).
	// Independiente de suggested_saves — una fase puede tener contrato de tools
	// sin tener required saves, y viceversa.
	if tc, ok := step.Inputs["required_tool_calls"].([]any); ok {
		for _, raw := range tc {
			if s, ok := raw.(string); ok && s != "" {
				out.RequiredToolCalls = append(out.RequiredToolCalls, s)
			}
		}
	}
	saves, ok := step.Inputs["suggested_saves"].([]any)
	if !ok {
		return out
	}
	for _, raw := range saves {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		s := phases.SuggestedSave{}
		if t, ok := m["type"].(string); ok {
			s.Type = t
		}
		if r, ok := m["required"].(bool); ok {
			s.Required = r
		}
		if h, ok := m["hint"].(string); ok {
			s.Hint = h
		}
		out.SuggestedSaves = append(out.SuggestedSaves, s)
	}
	return out
}

// missingToolCalls resuelve el contrato efectivo de tools de la fase y devuelve
// las que faltan respecto de lo reportado por el cliente (REQ-54 issue-54.1).
//
// Contrato efectivo: el override de agent_templates.metadata.required_tool_calls
// gana si está presente y no vacío; si no, el default del handler (out). Si el
// contrato resultante está vacío, devuelve nil (no-op, retrocompat).
func (s *Service) missingToolCalls(ctx context.Context, orgID uuid.UUID, slug phases.PhaseSlug, out *phases.Output, reported []string) []string {
	var contract []string
	if out != nil {
		contract = out.RequiredToolCalls
	}
	// Override desde BD (si el repo lo tiene y hay template para el slug).
	if s.Repo != nil {
		if tmpl, err := s.Repo.GetAgentTemplate(ctx, orgID, string(slug)); err == nil && tmpl != nil {
			if override := tmpl.RequiredToolCalls(); len(override) > 0 {
				contract = override
			}
		}
	}
	return missingFromContract(contract, reported)
}

// missingFromContract es la lógica pura de subconjunto: devuelve los elementos de
// contract que no están en reported. Contrato vacío → nil. Testeable sin BD.
func missingFromContract(contract, reported []string) []string {
	if len(contract) == 0 {
		return nil
	}
	have := make(map[string]struct{}, len(reported))
	for _, r := range reported {
		have[r] = struct{}{}
	}
	var missing []string
	for _, c := range contract {
		if _, ok := have[c]; !ok {
			missing = append(missing, c)
		}
	}
	return missing
}
