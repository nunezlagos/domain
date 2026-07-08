package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/metrics"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// fakeRepo es un Repository in-memory minimalista — sólo para verificar
// que la lógica de métricas se dispara. NO replica el comportamiento
// completo (eso lo cubren los integration tests).
type fakeRepo struct {
	flowRun  *FlowRunRow
	step     *FlowRunStepRow
	allSteps []FlowRunStepRow
	markedCompleted bool
	markedFailed    bool
}

func (f *fakeRepo) GetFlowIDBySlug(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (f *fakeRepo) CreateFlowRun(_ context.Context, _ FlowRunInsert) error { return nil }
func (f *fakeRepo) CreateFlowRunStep(_ context.Context, _ FlowRunStepInsert) error { return nil }
func (f *fakeRepo) GetFlowRun(_ context.Context, _ uuid.UUID) (*FlowRunRow, error) {
	return f.flowRun, nil
}
func (f *fakeRepo) GetFlowRunStep(_ context.Context, _ uuid.UUID) (*FlowRunStepRow, error) {
	return f.step, nil
}
func (f *fakeRepo) ListFlowRunSteps(_ context.Context, _ uuid.UUID) ([]FlowRunStepRow, error) {
	return f.allSteps, nil
}
func (f *fakeRepo) MarkStepCompleted(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	f.markedCompleted = true
	return nil
}
func (f *fakeRepo) MarkStepFailed(_ context.Context, _ uuid.UUID, _ string) error {
	f.markedFailed = true
	return nil
}

func (f *fakeRepo) SetFlowRunError(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (f *fakeRepo) UpdateFlowRunStatus(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (f *fakeRepo) UpdateStepInputs(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return nil
}
func (f *fakeRepo) MarkStepBlocked(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (f *fakeRepo) MarkStepPending(_ context.Context, _ uuid.UUID) error           { return nil }
func (f *fakeRepo) MarkStepCancelled(_ context.Context, _ uuid.UUID) error         { return nil }
func (f *fakeRepo) GetAgentTemplateSystemPrompt(_ context.Context, _ uuid.UUID, _ string) (string, error) {
	return "system", nil
}
func (f *fakeRepo) GetAgentTemplate(_ context.Context, _ uuid.UUID, slug string) (*AgentTemplate, error) {
	return &AgentTemplate{Slug: slug, Model: "claude-sonnet-4-6", Temperature: 0.3, MaxTokens: 4096, SystemPrompt: "system"}, nil
}

func TestService_RecordPhaseResult_IncrementsCompletedMetric(t *testing.T) {
	t.Parallel()
	reg := metrics.New()
	stepID := uuid.New()
	flowRunID := uuid.New()
	repo := &fakeRepo{
		flowRun: &FlowRunRow{ID: flowRunID, Status: "running"},
		step: &FlowRunStepRow{
			ID: stepID, FlowRunID: flowRunID, StepKey: "sdd-apply", Status: "pending",
			Inputs: map[string]any{
				"mode": "express",
				"suggested_saves": []any{
					map[string]any{"type": "code_reference", "required": true},
				},
			},
		},
		allSteps: []FlowRunStepRow{
			{ID: stepID, StepKey: "sdd-apply", Status: "completed"},
		},
	}
	s := &Service{Repo: repo, Phases: phases.NewRegistry(), Metrics: reg}

	_, err := s.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID:   stepID,
		Output:          map[string]any{"summary": "done"},
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
		DurationMS:      1500,
	})
	require.NoError(t, err)
	require.True(t, repo.markedCompleted)

	val := testMetricValue(t, reg, "domain_orchestrator_phase_results_total",
		map[string]string{"phase": "sdd-apply", "mode": "express", "result": "completed"})
	require.Equal(t, 1.0, val)
}

func TestService_RecordPhaseResult_IncrementsRequiredSaveMissingMetric(t *testing.T) {
	t.Parallel()
	reg := metrics.New()
	stepID := uuid.New()
	flowRunID := uuid.New()
	repo := &fakeRepo{
		flowRun: &FlowRunRow{ID: flowRunID, Status: "running"},
		step: &FlowRunStepRow{
			ID: stepID, FlowRunID: flowRunID, StepKey: "sdd-apply", Status: "pending",
			Inputs: map[string]any{
				"mode": "express",
				"suggested_saves": []any{
					map[string]any{"type": "code_reference", "required": true},
				},
			},
		},
		allSteps: []FlowRunStepRow{
			{ID: stepID, StepKey: "sdd-apply", Status: "running"},
		},
	}
	s := &Service{Repo: repo, Phases: phases.NewRegistry(), Metrics: reg}

	// REQ-56 issue-56.5: un save requerido faltante YA NO es un error hard ni mata
	// el step. Queda running (reintentable) y devuelve missing_required_saves. La
	// métrica de saves faltantes sigue incrementándose para observabilidad.
	res, err := s.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID: stepID,
		Output:        map[string]any{"summary": "no save"},
	})
	require.NoError(t, err)
	require.False(t, repo.markedFailed, "el step no debe marcarse failed")
	require.Equal(t, "pending", res.StepStatus, "el step queda reintentable")
	require.Len(t, res.MissingRequiredSaves, 1)
	require.Equal(t, "code_reference", res.MissingRequiredSaves[0].Type)

	val := testMetricValue(t, reg, "domain_orchestrator_required_save_missing_total",
		map[string]string{"phase": "sdd-apply", "save_type": "code_reference"})
	require.Equal(t, 1.0, val)
}

func TestService_ConfirmContinue_IncrementsConfirmsMetric(t *testing.T) {
	t.Parallel()
	reg := metrics.New()
	stepID := uuid.New()
	flowRunID := uuid.New()
	repo := &fakeRepo{
		flowRun: &FlowRunRow{ID: flowRunID, Status: "running"},
		allSteps: []FlowRunStepRow{
			{ID: stepID, StepKey: "sdd-verify", Status: "blocked",
				Inputs: map[string]any{"user_prompt": "verify..."}},
		},
	}
	s := &Service{Repo: repo, Phases: phases.NewRegistry(), Metrics: reg}

	_, err := s.ConfirmContinue(context.Background(), flowRunID, true)
	require.NoError(t, err)
	val := testMetricValue(t, reg, "domain_orchestrator_confirms_total",
		map[string]string{"confirmed": "true"})
	require.Equal(t, 1.0, val)
}

// testMetricValue usa el testutil de prometheus/client_golang para leer
// el valor actual de un counter por nombre + labels.
func testMetricValue(t *testing.T, reg *metrics.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	switch name {
	case "domain_orchestrator_phase_results_total":
		m, err := reg.OrchestratorPhaseResultsTotal.GetMetricWith(labels)
		require.NoError(t, err)
		return testutil.ToFloat64(m)
	case "domain_orchestrator_required_save_missing_total":
		m, err := reg.OrchestratorRequiredSaveMissingTotal.GetMetricWith(labels)
		require.NoError(t, err)
		return testutil.ToFloat64(m)
	case "domain_orchestrator_confirms_total":
		m, err := reg.OrchestratorConfirmsTotal.GetMetricWith(labels)
		require.NoError(t, err)
		return testutil.ToFloat64(m)
	}
	t.Fatalf("unknown metric: %s", name)
	return 0
}
