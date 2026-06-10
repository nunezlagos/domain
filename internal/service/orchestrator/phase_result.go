package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// PhaseResultInput es lo que el cliente IDE reporta vía MCP cuando
// termina una fase. El service lo valida (D5 saves + handler.Validate)
// y persiste el resultado.
type PhaseResultInput struct {
	FlowRunStepID   uuid.UUID
	Output          map[string]any
	MemoryRefsSaved []phases.MemoryRef
	DurationMS      int64
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
	if s.Repo == nil {
		return nil, errors.New("orchestrator: Repo not configured")
	}
	step, err := s.Repo.GetFlowRunStep(ctx, in.FlowRunStepID)
	if err != nil {
		return nil, err
	}
	if step.Status != "pending" && step.Status != "running" {
		return nil, ErrFlowRunStepNotPending
	}
	flowRun, err := s.Repo.GetFlowRun(ctx, step.FlowRunID)
	if err != nil {
		return nil, err
	}

	// Reconstruir el Output del handler para validación: se persistió
	// en step.inputs.suggested_saves al crear el step. Esto evita que
	// el handler necesite rebuild el prompt completo cada vez.
	rebuilt := rebuildOutputFromStepInputs(step)
	phaseSlug := phases.PhaseSlug(step.StepKey)

	// Validación D5 (centralizada)
	if err := ValidateRequiredSaves(phaseSlug, rebuilt,
		phases.ClientResult{Output: in.Output, MemoryRefsSaved: in.MemoryRefsSaved}); err != nil {
		// Marcar step como failed; propagar agregado al flow_run para
		// que GetFlowStatus refleje el estado terminal. El cliente
		// puede re-emitir con los saves correctos pero el flow ya
		// quedó marcado failed (D5 es bloqueante por diseño).
		_ = s.Repo.MarkStepFailed(ctx, step.ID, err.Error())
		_ = s.propagateFlowStatusAfterFailure(ctx, flowRun.ID)
		return nil, err
	}

	// Validación shape-specific del handler concreto
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

	// Persistir resultado
	if err := s.Repo.MarkStepCompleted(ctx, step.ID, in.Output); err != nil {
		return nil, fmt.Errorf("mark completed: %w", err)
	}

	// Calcular status agregado del flow_run + next step si hay
	steps, err := s.Repo.ListFlowRunSteps(ctx, flowRun.ID)
	if err != nil {
		return nil, fmt.Errorf("list steps for status: %w", err)
	}
	out := &PhaseResultResult{
		StepID:        step.ID,
		StepStatus:    "completed",
		FlowRunStatus: flowRun.Status,
	}
	out.FlowRunStatus, out.NextStepID, out.NextStepKey = aggregateFlowStatus(steps)
	if out.FlowRunStatus != flowRun.Status {
		if err := s.Repo.UpdateFlowRunStatus(ctx, flowRun.ID, out.FlowRunStatus); err != nil {
			return nil, fmt.Errorf("update flow_run status: %w", err)
		}
	}
	// Next step prompt: leemos del step.inputs.user_prompt persistido
	if out.NextStepID != nil {
		for _, st := range steps {
			if st.ID == *out.NextStepID {
				if up, ok := st.Inputs["user_prompt"].(string); ok {
					out.NextStepPrompt = up
				}
				break
			}
		}
	}
	return out, nil
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
	return s.Repo.UpdateFlowRunStatus(ctx, flowRunID, newStatus)
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
			// terminal
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

// rebuildOutputFromStepInputs reconstruye un phases.Output desde el
// JSONB persistido en flow_run_steps.inputs. Sólo necesitamos los
// suggested_saves para D5 validation; el system/user prompt no se
// re-valida (eso ya pasó en handler.Build).
func rebuildOutputFromStepInputs(step *FlowRunStepRow) *phases.Output {
	out := &phases.Output{}
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
