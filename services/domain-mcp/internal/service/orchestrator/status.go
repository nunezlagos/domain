package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/flow"
)

// FlowStatusResponse es la vista pública del estado de un flow_run
// gobernado por el orquestador. Lo consume domain_flow_status (mcp-004).
type FlowStatusResponse struct {
	FlowRunID         uuid.UUID         `json:"flow_run_id"`
	OrchestratorRunID string            `json:"orchestrator_run_id,omitempty"`
	Mode              string            `json:"mode,omitempty"`
	Status            string            `json:"status"`
	Steps             []FlowStepStatus  `json:"steps"`
}

// FlowStepStatus es el estado individual de un step. Omite los blobs
// grandes (system_prompt, user_prompt) para mantener la respuesta del
// status liviana — el cliente puede pedir el detalle con
// domain_orchestrate_phase_result lookup separado si lo necesita.
type FlowStepStatus struct {
	StepID           uuid.UUID      `json:"step_id"`
	StepKey          string         `json:"step_key"`
	Status           string         `json:"status"`
	Attempt          int            `json:"attempt"`
	Error            string         `json:"error,omitempty"`
	Outputs          map[string]any `json:"outputs,omitempty"`
	UserPromptPreview string        `json:"user_prompt_preview,omitempty"`
}

// GetFlowStatus devuelve el estado completo de un flow_run para que el
// cliente IDE pueda renderizar UI de progreso, decidir si reanudar, etc.
func (s *Service) GetFlowStatus(ctx context.Context, flowRunID uuid.UUID) (*FlowStatusResponse, error) {
	if s.Repo == nil {
		return nil, errors.New("orchestrator: Repo not configured")
	}
	flowRun, err := s.Repo.GetFlowRun(ctx, flowRunID)
	if err != nil {
		return nil, err
	}
	steps, err := s.Repo.ListFlowRunSteps(ctx, flowRunID)
	if err != nil {
		return nil, err
	}
	resp := &FlowStatusResponse{
		FlowRunID: flowRun.ID,
		Status:    flowRun.Status,
	}
	if orchID, ok := flowRun.Cursor["orchestrator_run_id"].(string); ok {
		resp.OrchestratorRunID = orchID
	}
	if mode, ok := flowRun.Cursor["mode"].(string); ok {
		resp.Mode = mode
	}
	resp.Steps = make([]FlowStepStatus, 0, len(steps))
	for _, st := range steps {
		s := FlowStepStatus{
			StepID:  st.ID,
			StepKey: st.StepKey,
			Status:  st.Status,
			Attempt: st.Attempt,
			Error:   st.Error,
			Outputs: st.Outputs,
		}
		if up, ok := st.Inputs["user_prompt"].(string); ok {
			s.UserPromptPreview = truncatePreview(up, 200)
		}
		resp.Steps = append(resp.Steps, s)
	}
	return resp, nil
}

// truncatePreview corta strings largos para responses de status sin
// matar el contexto del cliente — el prompt completo está disponible
// vía lookup del step si hace falta.
func truncatePreview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// CancelFlow lleva un flow_run a estado terminal 'cancelled' cuando el
// trabajo ya no aplica (p.ej. la feature fue retirada). Valida la
// transición contra la state machine (running/paused/pending → cancelled
// son legales; un flow ya terminal la rechaza) y persiste el motivo en
// flow_runs.error para dejar audit trail. Devuelve el estado resultante.
func (s *Service) CancelFlow(ctx context.Context, flowRunID uuid.UUID, reason string) (*FlowStatusResponse, error) {
	if s.Repo == nil {
		return nil, errors.New("orchestrator: Repo not configured")
	}
	flowRun, err := s.Repo.GetFlowRun(ctx, flowRunID)
	if err != nil {
		return nil, err
	}
	sm := flow.NewFlowStateMachine()
	if err := sm.ValidateTransition(flow.FlowStatus(flowRun.Status), flow.FlowStatusCancelled); err != nil {
		return nil, fmt.Errorf("no se puede cancelar un flow en estado %q: %w", flowRun.Status, err)
	}
	// Cancela los steps aún no-terminales para que no queden huérfanos
	// en pending/running/blocked tras cerrar el flow.
	steps, err := s.Repo.ListFlowRunSteps(ctx, flowRunID)
	if err != nil {
		return nil, err
	}
	for _, st := range steps {
		if st.Status == "pending" || st.Status == "running" || st.Status == "blocked" {
			if err := s.Repo.MarkStepCancelled(ctx, st.ID); err != nil {
				return nil, err
			}
		}
	}
	if reason != "" {
		if err := s.Repo.SetFlowRunError(ctx, flowRunID, reason); err != nil {
			return nil, err
		}
	}
	if err := s.Repo.UpdateFlowRunStatus(ctx, flowRunID, string(flow.FlowStatusCancelled)); err != nil {
		return nil, err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "flow_run.cancelled",
			EntityType: "flow_run",
			EntityID:   &flowRunID,
			OldValues:  map[string]any{"status": flowRun.Status},
			NewValues:  map[string]any{"status": string(flow.FlowStatusCancelled), "reason": reason},
		})
	}
	return s.GetFlowStatus(ctx, flowRunID)
}
