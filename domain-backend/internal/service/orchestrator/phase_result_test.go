package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

func TestRebuildOutputFromStepInputs_PreservesD5Contract(t *testing.T) {
	t.Parallel()
	step := &FlowRunStepRow{
		Inputs: map[string]any{
			"suggested_saves": []any{
				map[string]any{
					"type":     "code_reference",
					"required": true,
					"hint":     "save the file modified",
				},
				map[string]any{
					"type":     "knowledge_doc",
					"required": false,
					"hint":     "",
				},
			},
		},
	}
	out := rebuildOutputFromStepInputs(step)
	require.Len(t, out.SuggestedSaves, 2)
	require.Equal(t, "code_reference", out.SuggestedSaves[0].Type)
	require.True(t, out.SuggestedSaves[0].Required)
	require.Equal(t, "save the file modified", out.SuggestedSaves[0].Hint)
	require.Equal(t, "knowledge_doc", out.SuggestedSaves[1].Type)
	require.False(t, out.SuggestedSaves[1].Required)
}

func TestRebuildOutputFromStepInputs_EmptyInputs(t *testing.T) {
	t.Parallel()
	step := &FlowRunStepRow{Inputs: map[string]any{}}
	out := rebuildOutputFromStepInputs(step)
	require.Empty(t, out.SuggestedSaves)
}

func TestRebuildOutputFromStepInputs_MalformedEntriesSkipped(t *testing.T) {
	t.Parallel()
	step := &FlowRunStepRow{
		Inputs: map[string]any{
			"suggested_saves": []any{
				"not-a-map",
				123,
				map[string]any{"type": "valid", "required": true},
			},
		},
	}
	out := rebuildOutputFromStepInputs(step)
	require.Len(t, out.SuggestedSaves, 1, "los entries malformados se ignoran sin crash")
	require.Equal(t, "valid", out.SuggestedSaves[0].Type)
}

func TestAggregateFlowStatus_AllPending(t *testing.T) {
	t.Parallel()
	id1 := uuid.New()
	steps := []FlowRunStepRow{
		{ID: id1, StepKey: "sdd-apply", Status: "pending"},
		{ID: uuid.New(), StepKey: "sdd-verify", Status: "pending"},
	}
	status, next, key := aggregateFlowStatus(steps)
	require.Equal(t, "running", status)
	require.NotNil(t, next)
	require.Equal(t, id1, *next)
	require.Equal(t, "sdd-apply", key)
}

func TestAggregateFlowStatus_AllCompleted(t *testing.T) {
	t.Parallel()
	steps := []FlowRunStepRow{
		{ID: uuid.New(), Status: "completed"},
		{ID: uuid.New(), Status: "completed"},
	}
	status, next, _ := aggregateFlowStatus(steps)
	require.Equal(t, "completed", status)
	require.Nil(t, next)
}

func TestAggregateFlowStatus_AnyFailed(t *testing.T) {
	t.Parallel()
	steps := []FlowRunStepRow{
		{ID: uuid.New(), Status: "completed"},
		{ID: uuid.New(), Status: "failed"},
		{ID: uuid.New(), Status: "pending"},
	}
	status, next, _ := aggregateFlowStatus(steps)
	require.Equal(t, "failed", status, "cualquier failed → flow failed (sin importar pending posteriores)")
	require.Nil(t, next, "flow failed no expone next step")
}

func TestAggregateFlowStatus_PartialProgress(t *testing.T) {
	t.Parallel()
	id2 := uuid.New()
	steps := []FlowRunStepRow{
		{ID: uuid.New(), StepKey: "sdd-apply", Status: "completed"},
		{ID: id2, StepKey: "sdd-verify", Status: "pending"},
	}
	status, next, key := aggregateFlowStatus(steps)
	require.Equal(t, "running", status)
	require.NotNil(t, next)
	require.Equal(t, id2, *next)
	require.Equal(t, "sdd-verify", key)
}

func TestExtractConcerns_ValidInput(t *testing.T) {
	t.Parallel()
	output := map[string]any{
		"multi_concern": true,
		"concerns": []any{
			map[string]any{"name": "auth", "description": "Implementar login JWT"},
			map[string]any{"name": "cache", "description": "Agregar Redis para sesiones"},
		},
	}
	concerns := extractConcerns(output)
	require.Len(t, concerns, 2)
	require.Equal(t, "auth", concerns[0].Name)
	require.Equal(t, "Implementar login JWT", concerns[0].Description)
	require.Equal(t, "cache", concerns[1].Name)
	require.Equal(t, "Agregar Redis para sesiones", concerns[1].Description)
}

func TestExtractConcerns_NoConcernsField(t *testing.T) {
	t.Parallel()
	output := map[string]any{"multi_concern": true}
	concerns := extractConcerns(output)
	require.Nil(t, concerns)
}

func TestExtractConcerns_EmptyConcerns(t *testing.T) {
	t.Parallel()
	output := map[string]any{"multi_concern": true, "concerns": []any{}}
	concerns := extractConcerns(output)
	require.Empty(t, concerns)
}

func TestExtractConcerns_MalformedEntrySkipped(t *testing.T) {
	t.Parallel()
	output := map[string]any{
		"multi_concern": true,
		"concerns": []any{
			"not-a-map",
			42,
			map[string]any{"name": "valid", "description": "ok"},
		},
	}
	concerns := extractConcerns(output)
	require.Len(t, concerns, 1)
	require.Equal(t, "valid", concerns[0].Name)
}

func TestExtractConcerns_EmptyNameSkipped(t *testing.T) {
	t.Parallel()
	output := map[string]any{
		"multi_concern": true,
		"concerns": []any{
			map[string]any{"name": "", "description": "no name"},
			map[string]any{"name": "ok", "description": "fine"},
		},
	}
	concerns := extractConcerns(output)
	require.Len(t, concerns, 1)
	require.Equal(t, "ok", concerns[0].Name)
}

func TestAggregateFlowStatus_SkippedCountsAsTerminal(t *testing.T) {
	t.Parallel()
	steps := []FlowRunStepRow{
		{ID: uuid.New(), Status: "completed"},
		{ID: uuid.New(), Status: "skipped"},
	}
	status, _, _ := aggregateFlowStatus(steps)
	require.Equal(t, "completed", status,
		"skipped + completed = flow completed (skipped es terminal igual que cancelled)")
}

// multiConcernRepo es un fake Repository minimalista para probar D2.
// Solo implementa los métodos que RecordPhaseResult necesita.
type multiConcernRepo struct {
	flowRun         *FlowRunRow
	step            *FlowRunStepRow
	allSteps        []FlowRunStepRow
	cancelledSteps  []uuid.UUID
	updatedStatusTo string
	completedID     uuid.UUID
}

func (m *multiConcernRepo) GetFlowIDBySlug(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (m *multiConcernRepo) CreateFlowRun(_ context.Context, _ FlowRunInsert) error { return nil }
func (m *multiConcernRepo) CreateFlowRunStep(_ context.Context, _ FlowRunStepInsert) error { return nil }
func (m *multiConcernRepo) GetFlowRun(_ context.Context, _ uuid.UUID) (*FlowRunRow, error) {
	return m.flowRun, nil
}
func (m *multiConcernRepo) GetFlowRunStep(_ context.Context, _ uuid.UUID) (*FlowRunStepRow, error) {
	return m.step, nil
}
func (m *multiConcernRepo) ListFlowRunSteps(_ context.Context, _ uuid.UUID) ([]FlowRunStepRow, error) {
	// Devolver una copia con status actualizados
	out := make([]FlowRunStepRow, len(m.allSteps))
	copy(out, m.allSteps)
	for i := range out {
		if out[i].ID == m.completedID {
			out[i].Status = "completed"
		}
		for _, cid := range m.cancelledSteps {
			if out[i].ID == cid {
				out[i].Status = "cancelled"
			}
		}
	}
	return out, nil
}
func (m *multiConcernRepo) MarkStepCompleted(_ context.Context, stepID uuid.UUID, _ map[string]any) error {
	m.completedID = stepID
	return nil
}
func (m *multiConcernRepo) MarkStepFailed(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *multiConcernRepo) UpdateFlowRunStatus(_ context.Context, _ uuid.UUID, status string) error {
	m.updatedStatusTo = status
	return nil
}
func (m *multiConcernRepo) UpdateStepInputs(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return nil
}
func (m *multiConcernRepo) MarkStepBlocked(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *multiConcernRepo) MarkStepPending(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *multiConcernRepo) MarkStepCancelled(_ context.Context, stepID uuid.UUID) error {
	m.cancelledSteps = append(m.cancelledSteps, stepID)
	return nil
}
func (m *multiConcernRepo) GetAgentTemplateSystemPrompt(_ context.Context, _ uuid.UUID, _ string) (string, error) {
	return "system", nil
}
func (m *multiConcernRepo) GetAgentTemplate(_ context.Context, _ uuid.UUID, _ string) (*AgentTemplate, error) {
	return &AgentTemplate{Slug: "sdd-explore"}, nil
}

func TestRecordPhaseResult_MultiConcern_CancelsRemainingSteps(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	applyStepID := uuid.New()
	orgID := uuid.New()

	repo := &multiConcernRepo{
		flowRun: &FlowRunRow{
			ID:             flowRunID,
			OrganizationID: orgID,
			Status:         "running",
		},
		step: &FlowRunStepRow{
			ID:         stepID,
			FlowRunID:  flowRunID,
			StepKey:    "sdd-explore",
			Status:     "running",
			Inputs:     map[string]any{},
		},
		allSteps: []FlowRunStepRow{
			{ID: stepID, FlowRunID: flowRunID, StepKey: "sdd-explore", Status: "running", Inputs: map[string]any{}},
			{ID: applyStepID, FlowRunID: flowRunID, StepKey: "sdd-apply", Status: "pending", Inputs: map[string]any{}},
		},
	}

	reg := phases.NewRegistry()
	reg.Register(phases.NewSDDExploreHandler())

	s := &Service{
		Repo:   repo,
		Phases: reg,
		Env:    "dev",
	}

	res, err := s.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID: stepID,
		Output: map[string]any{
			"intent":        "feature",
			"multi_concern": true,
			"concerns": []any{
				map[string]any{"name": "auth", "description": "Implementar login JWT"},
				map[string]any{"name": "cache", "description": "Agregar Redis para sesiones"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "completed", res.StepStatus)
	require.NotNil(t, res.MultiConcern)
	require.Len(t, res.MultiConcern.Concerns, 2)
	require.Equal(t, "auth", res.MultiConcern.Concerns[0].Name)
	require.Equal(t, "Implementar login JWT", res.MultiConcern.Concerns[0].Description)
	require.Equal(t, "cache", res.MultiConcern.Concerns[1].Name)

	// Verificar que el step apply fue cancelado
	require.Len(t, repo.cancelledSteps, 1)
	require.Equal(t, applyStepID, repo.cancelledSteps[0])

	// Flow status debería ser "completed" (explore completed + apply cancelled)
	require.Equal(t, "completed", res.FlowRunStatus)
}

func TestRecordPhaseResult_MultiConcern_NoConcerns_NoCrash(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()

	repo := &multiConcernRepo{
		flowRun: &FlowRunRow{
			ID:             flowRunID,
			OrganizationID: orgID,
			Status:         "running",
		},
		step: &FlowRunStepRow{
			ID:        stepID,
			FlowRunID: flowRunID,
			StepKey:   "sdd-explore",
			Status:    "running",
			Inputs:    map[string]any{},
		},
		allSteps: []FlowRunStepRow{
			{ID: stepID, FlowRunID: flowRunID, StepKey: "sdd-explore", Status: "running", Inputs: map[string]any{}},
		},
	}

	reg := phases.NewRegistry()
	reg.Register(phases.NewSDDExploreHandler())

	s := &Service{
		Repo:   repo,
		Phases: reg,
		Env:    "dev",
	}

	res, err := s.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID: stepID,
		Output: map[string]any{
			"intent":        "feature",
			"multi_concern": true,
			// sin concerns
		},
	})
	require.NoError(t, err)
	require.Equal(t, "completed", res.StepStatus)
	// Sin concerns válidos → MultiConcern nil
	require.Nil(t, res.MultiConcern)
}

func TestRecordPhaseResult_MultiConcern_NotForNonExplore(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()
	verifyStepID := uuid.New()

	repo := &multiConcernRepo{
		flowRun: &FlowRunRow{
			ID:             flowRunID,
			OrganizationID: orgID,
			Status:         "running",
		},
		step: &FlowRunStepRow{
			ID:        stepID,
			FlowRunID: flowRunID,
			StepKey:   "sdd-verify",
			Status:    "running",
			Inputs:    map[string]any{},
		},
		allSteps: []FlowRunStepRow{
			{ID: stepID, FlowRunID: flowRunID, StepKey: "sdd-verify", Status: "running", Inputs: map[string]any{}},
			{ID: verifyStepID, FlowRunID: flowRunID, StepKey: "sdd-archive", Status: "pending", Inputs: map[string]any{}},
		},
	}

	reg := phases.NewRegistry()
	reg.Register(phases.NewSDDVerifyHandler())

	s := &Service{
		Repo:   repo,
		Phases: reg,
		Env:    "dev",
	}

	// Verify necesita scenarios_failed, tests_passed, etc. Output válido
	// pero sin multi_concern real.
	res, err := s.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID: stepID,
		Output: map[string]any{
			"multi_concern":    true,
			"scenarios_failed": []any{},
			"tests_passed":     5,
			"tests_failed":     0,
		},
	})
	require.NoError(t, err)
	// Verify returns "completed" for valid output
	require.Equal(t, "completed", res.StepStatus)
	// Pero MultiConcern es nil porque el handler no es sdd-explore
	require.Nil(t, res.MultiConcern)
	// No se cancelaron steps
	require.Len(t, repo.cancelledSteps, 0)
}

func TestRecordPhaseResult_DualOutput_ExtractsSummary(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()

	repo := &multiConcernRepo{
		flowRun: &FlowRunRow{
			ID:             flowRunID,
			OrganizationID: orgID,
			Status:         "running",
		},
		step: &FlowRunStepRow{
			ID:        stepID,
			FlowRunID: flowRunID,
			StepKey:   "sdd-apply",
			Status:    "running",
			Inputs:    map[string]any{},
		},
		allSteps: []FlowRunStepRow{
			{ID: stepID, FlowRunID: flowRunID, StepKey: "sdd-apply", Status: "running", Inputs: map[string]any{}},
		},
	}

	reg := phases.NewRegistry()
	reg.Register(phases.NewSDDApplyHandler())

	s := &Service{
		Repo:   repo,
		Phases: reg,
		Env:    "dev",
	}

	res, err := s.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID: stepID,
		Output: map[string]any{
			"summary":      "Corregido typo en CHANGELOG.md",
			"files_changed": []any{"CHANGELOG.md"},
			"lines_changed": 1,
		},
		MemoryRefsSaved: []phases.MemoryRef{
			{Type: "code_reference", ID: uuid.New()},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "Corregido typo en CHANGELOG.md", res.Summary)
	require.Equal(t, "completed", res.StepStatus)
}

func TestRecordPhaseResult_DualOutput_NoSummary_DefaultsEmpty(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()

	repo := &multiConcernRepo{
		flowRun: &FlowRunRow{
			ID:             flowRunID,
			OrganizationID: orgID,
			Status:         "running",
		},
		step: &FlowRunStepRow{
			ID:        stepID,
			FlowRunID: flowRunID,
			StepKey:   "sdd-apply",
			Status:    "running",
			Inputs:    map[string]any{},
		},
		allSteps: []FlowRunStepRow{
			{ID: stepID, FlowRunID: flowRunID, StepKey: "sdd-apply", Status: "running", Inputs: map[string]any{}},
		},
	}

	reg := phases.NewRegistry()
	reg.Register(phases.NewSDDApplyHandler())

	s := &Service{
		Repo:   repo,
		Phases: reg,
		Env:    "dev",
	}

	res, err := s.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID: stepID,
		Output: map[string]any{
			"files_changed": []any{"CHANGELOG.md"},
			"lines_changed": 1,
		},
		MemoryRefsSaved: []phases.MemoryRef{
			{Type: "code_reference", ID: uuid.New()},
		},
	})
	require.NoError(t, err)
	require.Empty(t, res.Summary, "sin summary en output → Summary vacío")
}
