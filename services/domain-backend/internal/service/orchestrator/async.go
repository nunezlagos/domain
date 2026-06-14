package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// Signal names for async mode flow_signals.
const (
	SignalNameStepCompleted = "orchestrator:step_completed"
	SignalNameStepFailed    = "orchestrator:step_failed"
	SignalNameFlowCompleted = "orchestrator:flow_completed"
	SignalNameFlowFailed    = "orchestrator:flow_failed"
)

// runAsync es el handler de ModeAsync en Service.Run. Construye el plan
// Full, persiste flow_run + steps, y devuelve inmediatamente con los IDs
// para que el caller pueda trackear progreso via GetFlowStatus o
// flow_signals.
//
// La ejecución concreta la hace ProcessAsyncFlowRun (worker externo o
// goroutine) que reanuda los steps en orden y emite flow_signals.
func (s *Service) runAsync(ctx context.Context, in OrchestrateInput,
	flowID, flowRunID, orchestratorRunID uuid.UUID, plan *modes.PhasePlan,
) (*OrchestrateResult, error) {
	res := &OrchestrateResult{
		OrchestratorRunID: orchestratorRunID,
		FlowRunID:         flowRunID,
		Mode:              ModeAsync,
		StartedAt:         s.now(),
		Plan:              exportPlan(plan),
	}
	if len(plan.Steps) > 0 {
		res.SnapshotPrompt = plan.Steps[0].UserPrompt
	}
	if s.Metrics != nil {
		s.Metrics.OrchestratorRunsTotal.WithLabelValues("async", "started").Inc()
	}
	return res, nil
}

// ProcessAsyncFlowRun ejecuta los steps pendientes de un flow_run en modo
// async. Es el "worker que tail": itera los steps en orden, para cada uno:
//
//  1. Reconstruye el user_prompt (lazy build si no está cacheado)
//  2. Lookup agent_template (model, temperature, max_tokens, system_prompt)
//  3. Resuelve el provider LLM via ProviderForModel
//  4. Llama provider.Complete con system + user prompt
//  5. Parsea output JSON
//  6. handler.Validate + ValidateRequiredSaves
//  7. MarkStepCompleted
//  8. Emite flow_signal con el resultado
//  9. Acumula PriorOutputs para el próximo step
//
// Al terminar todos los steps, actualiza flow_run.status = completed y
// emite flow_completed signal. Si falla, marca step+flow como failed y
// emite failure signal.
//
// Requiere s.LLM != nil (mismo requisito que Solo mode). Puede invocarse
// desde una goroutine lanzada por el MCP tool o desde un worker externo.
func (s *Service) ProcessAsyncFlowRun(ctx context.Context, flowRunID uuid.UUID) error {
	if s.LLM == nil {
		return ErrLLMFactoryRequired
	}
	if s.Repo == nil {
		return errors.New("orchestrator: Repo not configured")
	}

	flowRun, err := s.Repo.GetFlowRun(ctx, flowRunID)
	if err != nil {
		return fmt.Errorf("async: get flow_run: %w", err)
	}
	mode, _ := flowRun.Cursor["mode"].(string)
	if mode != "async" {
		return ErrAsyncFlowNotAsync
	}

	steps, err := s.Repo.ListFlowRunSteps(ctx, flowRunID)
	if err != nil {
		return fmt.Errorf("async: list steps: %w", err)
	}

	priors := map[phases.PhaseSlug]map[string]any{}
	// Reconstruir priors desde steps ya completados (por si es reanudación)
	for _, st := range steps {
		if st.Status == "completed" && len(st.Outputs) > 0 {
			priors[phases.PhaseSlug(st.StepKey)] = st.Outputs
		}
	}

	for i, step := range steps {
		if step.Status != "pending" && step.Status != "running" {
			continue
		}

		userPrompt, err := s.resolveAsyncStepPrompt(ctx, &steps[i], steps, priors)
		if err != nil {
			return fmt.Errorf("async: resolve prompt step %s: %w", step.StepKey, err)
		}

		slug := step.StepKey
		tmpl, err := s.Repo.GetAgentTemplate(ctx, flowRun.OrganizationID, slug)
		if err != nil {
			return fmt.Errorf("async: template %s: %w", slug, err)
		}

		provider, err := s.LLM.Get(modes.ProviderForModel(tmpl.Model))
		if err != nil {
			return fmt.Errorf("async: provider for %s: %w", tmpl.Model, err)
		}

		resp, err := provider.Complete(ctx, llm.CompletionOptions{
			Model:        tmpl.Model,
			Temperature:  float64(tmpl.Temperature),
			MaxTokens:    tmpl.MaxTokens,
			SystemPrompt: tmpl.SystemPrompt,
			Messages:     []llm.Message{{Role: "user", Content: userPrompt}},
		})
		if err != nil {
			_ = s.Repo.MarkStepFailed(ctx, step.ID, "async: llm complete: "+err.Error())
			_ = s.propagateFlowStatusAfterFailure(ctx, flowRunID)
			_ = s.emitSignal(ctx, flowRunID, &slug, SignalNameStepFailed,
				map[string]any{"step_key": slug, "error": err.Error()})
			return fmt.Errorf("async: complete step %s: %w", slug, err)
		}

		output, parseErr := parseJSONOutput(resp.Content)
		if parseErr != nil {
			_ = s.Repo.MarkStepFailed(ctx, step.ID, "async: invalid json: "+parseErr.Error())
			_ = s.propagateFlowStatusAfterFailure(ctx, flowRunID)
			_ = s.emitSignal(ctx, flowRunID, &slug, SignalNameStepFailed,
				map[string]any{"step_key": slug, "error": parseErr.Error()})
			return fmt.Errorf("async: parse step %s: %w", slug, parseErr)
		}

		h, err := s.Phases.Lookup(phases.PhaseSlug(slug))
		if err != nil {
			return fmt.Errorf("async: lookup %s: %w", slug, err)
		}
		if err := h.Validate(ctx, &phases.Output{}, phases.ClientResult{Output: output}); err != nil {
			_ = s.Repo.MarkStepFailed(ctx, step.ID, "async: validate: "+err.Error())
			_ = s.propagateFlowStatusAfterFailure(ctx, flowRunID)
			_ = s.emitSignal(ctx, flowRunID, &slug, SignalNameStepFailed,
				map[string]any{"step_key": slug, "error": err.Error()})
			return fmt.Errorf("async: validate step %s: %w", slug, err)
		}

		if err := s.Repo.MarkStepCompleted(ctx, step.ID, output); err != nil {
			return fmt.Errorf("async: mark completed %s: %w", slug, err)
		}
		priors[phases.PhaseSlug(slug)] = output

		if s.Metrics != nil {
			s.Metrics.OrchestratorPhaseResultsTotal.
				WithLabelValues(slug, "async", "completed").Inc()
		}

		if err := s.emitSignal(ctx, flowRunID, &slug, SignalNameStepCompleted,
			map[string]any{"step_key": slug, "status": "completed"}); err != nil {
			return fmt.Errorf("async: emit signal step %s: %w", slug, err)
		}
	}

	if err := s.Repo.UpdateFlowRunStatus(ctx, flowRunID, "completed"); err != nil {
		return fmt.Errorf("async: update flow_run status: %w", err)
	}
	if s.Metrics != nil {
		s.Metrics.OrchestratorRunsTotal.WithLabelValues("async", "completed").Inc()
	}
	if err := s.emitSignal(ctx, flowRunID, nil, SignalNameFlowCompleted,
		map[string]any{"status": "completed", "flow_run_id": flowRunID.String()}); err != nil {
		return fmt.Errorf("async: emit flow completed signal: %w", err)
	}
	return nil
}

// resolveAsyncStepPrompt obtiene el user_prompt para un step. Si el step
// ya tiene prompt cacheado en inputs, lo usa. Caso contrario, hace lazy
// build con los PriorOutputs acumulados (mismo patrón que Full mode).
func (s *Service) resolveAsyncStepPrompt(ctx context.Context,
	step *FlowRunStepRow, allSteps []FlowRunStepRow,
	priors map[phases.PhaseSlug]map[string]any,
) (string, error) {
	if cached, ok := step.Inputs["user_prompt"].(string); ok && cached != "" {
		return cached, nil
	}
	h, err := s.Phases.Lookup(phases.PhaseSlug(step.StepKey))
	if err != nil {
		return "", fmt.Errorf("resolve prompt lookup %s: %w", step.StepKey, err)
	}
	rawText := extractRawTextFromInputs(allSteps, step)
	out, err := h.Build(ctx, phases.Input{
		FlowRunID:    step.FlowRunID,
		PhaseSlug:    phases.PhaseSlug(step.StepKey),
		RawText:      rawText,
		PriorOutputs: priors,
	})
	if err != nil {
		return "", fmt.Errorf("resolve prompt build %s: %w", step.StepKey, err)
	}
	updatedInputs := mapClone(step.Inputs)
	updatedInputs["user_prompt"] = out.UserPrompt
	if err := s.Repo.UpdateStepInputs(ctx, step.ID, updatedInputs); err != nil {
		return "", fmt.Errorf("persist rebuilt prompt %s: %w", step.StepKey, err)
	}
	return out.UserPrompt, nil
}

// emitSignal emite un flow_signal. Si SignalStore no está configurado,
// es no-op (modo degraded sin señales no bloquea la ejecución).
func (s *Service) emitSignal(ctx context.Context, flowRunID uuid.UUID,
	stepKey *string, name string, payload map[string]any,
) error {
	if s.SignalStore == nil {
		return nil
	}
	var raw []byte
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal signal payload: %w", err)
		}
	}
	_, err := s.SignalStore.Send(ctx, flowRunID, stepKey, name, raw)
	return err
}
