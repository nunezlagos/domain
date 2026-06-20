package orchestrator

import (
	"context"
	"errors"

	"github.com/google/uuid"
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
