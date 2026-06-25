package orchestrator

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/metrics"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	skillsvc "nunezlagos/domain/internal/service/skill"
	"nunezlagos/domain/internal/tracing"

	"go.opentelemetry.io/otel/trace"
)

// Clock permite inyectar wall-clock para tests deterministas (regla
// .claude/rules/testing.md: nada de time.Now() directo en lógica).
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// Service coordina la ejecución del pipeline SDD. Responsable de:
//   - validar OrchestrateInput (modos, fases, combinaciones D6)
//   - resolver el DAG sdd-pipeline-v1 según Mode + SkipPhases
//   - crear flow_runs + flow_run_steps coherentes
//   - despachar fases vía phases.Registry (Build → cliente IDE → Validate)
//   - aplicar D5 (suggested_saves required) sobre los results del cliente
//
// La implementación concreta de Run() y de los modos vive en
// modes/{express,full,solo,detect,async}.go (svc-003..svc-007). Este
// archivo declara el skeleton público para que el resto del wiring
// (MCP tools, PromptRouter, CLI) pueda referenciarlo sin esperar a las
// fases.
type Service struct {
	Pool    *pgxpool.Pool
	Audit   audit.Recorder
	Phases  *phases.Registry




	Repo Repository


	Env string


	Clock Clock


	Metrics *metrics.Registry



	LLM *llm.Factory




	SignalStore *flow.SignalStore


	Skills *skillsvc.Service
}

// New construye un Service. El registry debe venir poblado por el
// caller (boot wiring) — el orquestador no se auto-registra fases para
// permitir testing con handlers fake.
//
// Si pool != nil, se construye un PGRepository automáticamente. Tests
// que quieran fakearlo pueden override `s.Repo` después de New.
func New(pool *pgxpool.Pool, audit audit.Recorder, reg *phases.Registry, env string) *Service {
	s := &Service{Pool: pool, Audit: audit, Phases: reg, Env: env, Clock: systemClock{}}
	if pool != nil {
		s.Repo = NewPGRepository(pool)
	}
	return s
}

// Run despacha el orquestador según el Mode. Devuelve OrchestrateResult
// con los IDs del flow_run creado (que el cliente IDE puede usar para
// pollear o reanudar) y el SnapshotPrompt si aplica.
//
// Estado de implementación por modo:
//   - Express: dispatcher in-memory completo (ya devuelve Plan ejecutable)
//   - Full / Solo / Detect / Async: stub temporal — devuelve IDs + Mode
//     sin Plan. Los modos restantes vienen en próximos chunks junto con
//     la persistencia flow_runs + dispatch loop MCP.
func (s *Service) Run(ctx context.Context, in OrchestrateInput) (*OrchestrateResult, error) {


	ctx, span := tracing.Tracer("orchestrator").Start(ctx, "orchestrator.run",
		trace.WithAttributes(
			tracing.SafeAttr("orchestrator.mode", string(in.Mode)),
			tracing.SafeAttr("org.id", in.OrganizationID.String()),
			tracing.SafeAttr("user.id", in.UserID.String()),
		),
	)
	defer span.End()

	if err := s.validate(in); err != nil {
		span.RecordError(err)
		return nil, err
	}
	mode := in.Mode
	if mode == "" {
		sig := analyzeComplexity(in.RawText)
		mode = decideMode(sig, in, s.LLM != nil)
	}
	now := s.now()
	res := &OrchestrateResult{
		OrchestratorRunID: uuid.New(),
		FlowRunID:         uuid.New(),
		Mode:              mode,
		StartedAt:         now,
	}
	if mode == ModeSolo {



		if s.Repo == nil {
			return nil, errors.New("orchestrator: Repo required for Solo mode")
		}
		flowID, err := s.Repo.GetFlowIDBySlug(ctx, in.OrganizationID, "sdd-pipeline-v1")
		if err != nil {
			return nil, err
		}
		plan, err := modes.BuildFullPlan(ctx, s.Phases, phases.Input{
			OrganizationID: in.OrganizationID,
			UserID:         in.UserID,
			FlowRunID:      res.FlowRunID,
			RawText:        in.RawText,
		}, phases.PhaseSlug(in.StartingPhase),
			convertSkipPhases(in.SkipPhases), now)
		if err != nil {
			return nil, err
		}
		if err := s.hydrateSystemPrompts(ctx, in.OrganizationID, in.ProjectID, plan); err != nil {
			return nil, err
		}
		if err := s.persistPlan(ctx, in, mode,
			res.OrchestratorRunID, flowID, res.FlowRunID, plan, now); err != nil {
			return nil, err
		}
		if s.Metrics != nil {
			s.Metrics.OrchestratorRunsTotal.WithLabelValues(string(mode), "started").Inc()
		}
		if err := s.runSolo(ctx, in, flowID, res.FlowRunID, res.OrchestratorRunID, plan); err != nil {
			return nil, err
		}
		res.Plan = exportPlan(plan)
		span.SetAttributes(
			tracing.SafeAttr("orchestrator.run_id", res.OrchestratorRunID.String()),
			tracing.SafeAttr("flow_run.id", res.FlowRunID.String()),
		)
		return res, nil
	}
	if mode == ModeAsync {
		if s.Repo == nil {
			return nil, errors.New("orchestrator: Repo required for Async mode")
		}
		flowID, err := s.Repo.GetFlowIDBySlug(ctx, in.OrganizationID, "sdd-pipeline-v1")
		if err != nil {
			return nil, err
		}
		plan, err := modes.BuildAsyncPlan(ctx, s.Phases, phases.Input{
			OrganizationID: in.OrganizationID,
			UserID:         in.UserID,
			FlowRunID:      res.FlowRunID,
			RawText:        in.RawText,
		}, phases.PhaseSlug(in.StartingPhase),
			convertSkipPhases(in.SkipPhases), now)
		if err != nil {
			return nil, err
		}
		if err := s.hydrateSystemPrompts(ctx, in.OrganizationID, in.ProjectID, plan); err != nil {
			return nil, err
		}
		if err := s.persistPlan(ctx, in, mode,
			res.OrchestratorRunID, flowID, res.FlowRunID, plan, now); err != nil {
			return nil, err
		}
		result, err := s.runAsync(ctx, in, flowID, res.FlowRunID, res.OrchestratorRunID, plan)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		span.SetAttributes(
			tracing.SafeAttr("orchestrator.run_id", result.OrchestratorRunID.String()),
			tracing.SafeAttr("flow_run.id", result.FlowRunID.String()),
		)
		return result, nil
	}
	if mode == ModeExpress || mode == ModeLite || mode == ModeFull || mode == ModeDetect {





		var flowID uuid.UUID
		if s.Repo != nil {
			var err error
			flowID, err = s.Repo.GetFlowIDBySlug(ctx, in.OrganizationID, "sdd-pipeline-v1")
			if err != nil {
				return nil, err
			}
		}
		phaseInput := phases.Input{
			OrganizationID: in.OrganizationID,
			UserID:         in.UserID,
			FlowRunID:      res.FlowRunID,
			RawText:        in.RawText,
		}
		var (
			plan *modes.PhasePlan
			err  error
		)
		switch mode {
		case ModeExpress:
			plan, err = modes.BuildExpressPlan(ctx, s.Phases, phaseInput, now)
		case ModeLite:




			plan, err = modes.BuildLitePlan(ctx, s.Phases, phaseInput, now)
		case ModeFull, ModeDetect:


			plan, err = modes.BuildFullPlan(ctx, s.Phases, phaseInput,
				phases.PhaseSlug(in.StartingPhase),
				convertSkipPhases(in.SkipPhases), now)
		}
		if err != nil {
			return nil, err
		}






		if s.Repo != nil {
			if err := s.hydrateSystemPrompts(ctx, in.OrganizationID, in.ProjectID, plan); err != nil {
				return nil, err
			}
		}




		if s.Repo != nil && mode != ModeDetect {
			if err := s.persistPlan(ctx, in, mode,
				res.OrchestratorRunID, flowID, res.FlowRunID, plan, now); err != nil {
				return nil, err
			}
		}
		res.Plan = exportPlan(plan)
		if len(res.Plan.Steps) > 0 {
			res.SnapshotPrompt = res.Plan.Steps[0].UserPrompt
		}




		if s.Metrics != nil {
			s.Metrics.OrchestratorRunsTotal.WithLabelValues(string(mode), "started").Inc()
		}
		span.SetAttributes(
			tracing.SafeAttr("orchestrator.run_id", res.OrchestratorRunID.String()),
			tracing.SafeAttr("flow_run.id", res.FlowRunID.String()),
		)
	}
	return res, nil
}

// convertSkipPhases pasa el slice del API público al tipo del subpaquete
// phases sin reexportar el tipo desde el service.
func convertSkipPhases(in []PhaseSlug) []phases.PhaseSlug {
	if len(in) == 0 {
		return nil
	}
	out := make([]phases.PhaseSlug, len(in))
	for i, p := range in {
		out[i] = phases.PhaseSlug(p)
	}
	return out
}

// exportPlan traduce el plan interno (modes.PhasePlan) al shape
// exportado (PhasePlanSummary). Mantiene aislado el package modes/
// del API público del service.
func exportPlan(p *modes.PhasePlan) *PhasePlanSummary {
	if p == nil {
		return nil
	}
	out := &PhasePlanSummary{Mode: p.Mode, Steps: make([]PhaseStepSummary, len(p.Steps))}
	for i, st := range p.Steps {
		saves := make([]SuggestedSaveSummary, len(st.SuggestedSaves))
		for j, s := range st.SuggestedSaves {
			saves[j] = SuggestedSaveSummary{Type: s.Type, Required: s.Required, Hint: s.Hint}
		}
		out.Steps[i] = PhaseStepSummary{
			ID:                st.ID,
			Slug:              PhaseSlug(st.Slug),
			AgentTemplateSlug: st.AgentTemplateSlug,
			SystemPrompt:      st.SystemPrompt,
			UserPrompt:        st.UserPrompt,
			SuggestedSaves:    saves,
			RetryPolicy:       string(st.RetryPolicy),
			SkillThreshold:    st.SkillThreshold,
		}
	}
	return out
}

// hydrateSystemPrompts rellena step.SystemPrompt para cada step del plan
// desde agent_templates en BD. Si el lookup falla con
// ErrAgentTemplateNotFound, devolvemos error para que el caller corrija
// el seed (no hay default sano para un prompt en blanco).
//
// También extrae SkillThreshold desde agent_templates.metadata (D3).
func (s *Service) hydrateSystemPrompts(ctx context.Context, orgID, projectID uuid.UUID, plan *modes.PhasePlan) error {
	if plan == nil {
		return nil
	}


	rulesBlock := s.buildRulesBlock(ctx, projectID)
	type cached struct {
		systemPrompt string
		threshold    float64
	}
	cache := make(map[string]cached, len(plan.Steps))
	for i := range plan.Steps {
		slug := plan.Steps[i].AgentTemplateSlug
		if slug == "" {
			continue
		}
		if c, ok := cache[slug]; ok {
			plan.Steps[i].SystemPrompt = c.systemPrompt
			plan.Steps[i].SkillThreshold = c.threshold
			continue
		}
		t, err := s.Repo.GetAgentTemplate(ctx, orgID, slug)
		if err != nil {
			return err
		}
		sysPrompt := t.SystemPrompt + rulesBlock
		c := cached{systemPrompt: sysPrompt, threshold: t.SkillThreshold()}
		cache[slug] = c
		plan.Steps[i].SystemPrompt = c.systemPrompt
		plan.Steps[i].SkillThreshold = c.threshold
	}
	return nil
}

// buildRulesBlock arma un bloque markdown con las reglas vigentes: las de
// plataforma (platform_policies) + las del proyecto (project_policies),
// respetando override_platform (si una policy de proyecto de un `kind`
// marca override, se omiten las de plataforma de ese mismo kind). Best-effort:
// si falla la lectura, devuelve "" (las reglas son aditivas, no bloqueantes).
func (s *Service) buildRulesBlock(ctx context.Context, projectID uuid.UUID) string {
	if s.Pool == nil {
		return ""
	}
	type pol struct {
		name, body, kind string
		override         bool
	}
	var platform, project []pol

	if rows, err := s.Pool.Query(ctx,
		`SELECT name, COALESCE(body_md,''), COALESCE(kind,'')
		   FROM platform_policies WHERE is_active = TRUE
		   ORDER BY kind, slug`); err == nil {
		for rows.Next() {
			var p pol
			if rows.Scan(&p.name, &p.body, &p.kind) == nil && p.body != "" {
				platform = append(platform, p)
			}
		}
		rows.Close()
	}

	if projectID != uuid.Nil {
		if rows, err := s.Pool.Query(ctx,
			`SELECT name, COALESCE(body_md,''), COALESCE(kind,''), override_platform
			   FROM project_policies
			   WHERE project_id = $1 AND is_active = TRUE
			     AND deleted_at IS NULL AND proposed = FALSE
			   ORDER BY kind, slug`, projectID); err == nil {
			for rows.Next() {
				var p pol
				if rows.Scan(&p.name, &p.body, &p.kind, &p.override) == nil && p.body != "" {
					project = append(project, p)
				}
			}
			rows.Close()
		}
	}

	overridden := make(map[string]bool)
	for _, p := range project {
		if p.override && p.kind != "" {
			overridden[p.kind] = true
		}
	}

	var b strings.Builder
	write := func(p pol) {
		b.WriteString("\n### ")
		b.WriteString(p.name)
		b.WriteString("\n")
		b.WriteString(p.body)
		b.WriteString("\n")
	}
	for _, p := range platform {
		if !overridden[p.kind] {
			write(p)
		}
	}
	for _, p := range project {
		write(p)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n\n## Reglas vigentes (plataforma + proyecto)\n" + b.String()
}

// now devuelve la hora vía Clock o cae a UTC system si Clock fue nil
// (caso constructores que no usaron New).
func (s *Service) now() time.Time {
	if s.Clock == nil {
		return time.Now().UTC()
	}
	return s.Clock.Now()
}

// validate aplica las reglas del contrato (no del DAG):
//
//   - RawText no vacío (ErrEmptyRawText)
//   - Mode válido (ErrInvalidMode)
//   - D6: ModeAsync + ModeExpress = ErrAsyncModeUnsupported
//   - StartingPhase y SkipPhases referencian fases registradas
//
// La validación del DAG resultante (¿el SkipPhases deja un grafo
// ejecutable?) ocurre en modes/validator.go (svc-008).
func (s *Service) validate(in OrchestrateInput) error {
	if strings.TrimSpace(in.RawText) == "" {
		return ErrEmptyRawText
	}


	if in.ProjectID == uuid.Nil {
		return ErrProjectIDRequired
	}
	if in.Mode != "" && !in.Mode.IsValid() {
		return ErrInvalidMode
	}
	switch in.ExecMode {
	case "", "auto", "manual", "hybrid":
	default:
		return ErrInvalidExecMode
	}
	if in.Mode == ModeAsync && in.ExpressMaxLines > 0 {


		return ErrAsyncModeUnsupported
	}
	if s.Phases != nil {
		if err := s.validatePhase(in.StartingPhase); err != nil {
			return err
		}
		for _, p := range in.SkipPhases {
			if err := s.validatePhase(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// validatePhase tolera el zero value (sin override del default).
func (s *Service) validatePhase(p PhaseSlug) error {
	if p == "" {
		return nil
	}
	_, err := s.Phases.Lookup(phases.PhaseSlug(p))
	if err != nil {
		if errors.Is(err, phases.ErrPhaseNotRegistered) {
			return ErrUnknownPhase
		}
		return err
	}
	return nil
}
