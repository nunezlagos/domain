package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// --- Lógica pura del contrato (sin BD) — REQ-54 issue-54.1 ---

func TestMissingFromContract_EmptyContract_NoOp(t *testing.T) {
	t.Parallel()
	// Contrato vacío = retrocompat: nunca falta nada, aunque el cliente no reporte.
	require.Nil(t, missingFromContract(nil, nil))
	require.Nil(t, missingFromContract([]string{}, nil))
	require.Nil(t, missingFromContract(nil, []string{"domain_verify_start"}))
}

func TestMissingFromContract_Missing_ReturnsGap(t *testing.T) {
	t.Parallel()
	missing := missingFromContract(
		[]string{"domain_verify_start", "domain_verify_complete"},
		[]string{"domain_verify_start"},
	)
	require.Equal(t, []string{"domain_verify_complete"}, missing)
}

func TestMissingFromContract_Complete_ReturnsNil(t *testing.T) {
	t.Parallel()
	// Reporte superset del contrato → nada falta.
	missing := missingFromContract(
		[]string{"domain_verify_start", "domain_verify_complete"},
		[]string{"domain_verify_start", "domain_verify_complete", "domain_mem_save"},
	)
	require.Nil(t, missing)
}

// --- Integración vía RecordPhaseResult — REQ-54 issue-54.1 ---

// toolContractRepo es un fake minimalista que permite inyectar el contrato de
// tools por dos vías: el default del handler (via step.Inputs -> rebuilt) y el
// override de agent_templates (templateContract).
type toolContractRepo struct {
	multiConcernRepo
	templateContract []string
}

func (r *toolContractRepo) GetAgentTemplate(_ context.Context, _ uuid.UUID, slug string) (*AgentTemplate, error) {
	tmpl := &AgentTemplate{Slug: slug}
	if len(r.templateContract) > 0 {
		arr := make([]any, len(r.templateContract))
		for i, s := range r.templateContract {
			arr[i] = s
		}
		tmpl.Metadata = map[string]any{"required_tool_calls": arr}
	}
	return tmpl, nil
}

// newToolContractStep arma un step de sdd-verify cuyo default de contrato
// (required_tool_calls en los Inputs, que rebuildOutputFromStepInputs preserva)
// es el pasado. Vacío = sin default.
func newToolContractStep(stepID, flowRunID uuid.UUID, defaultContract []string) *FlowRunStepRow {
	inputs := map[string]any{}
	if len(defaultContract) > 0 {
		arr := make([]any, len(defaultContract))
		for i, s := range defaultContract {
			arr[i] = s
		}
		inputs["required_tool_calls"] = arr
	}
	return &FlowRunStepRow{
		ID:        stepID,
		FlowRunID: flowRunID,
		StepKey:   "sdd-verify",
		Status:    "running",
		Inputs:    inputs,
	}
}

func TestRecordPhaseResult_ToolContract_Missing_RejectsPhase(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()

	repo := &toolContractRepo{
		multiConcernRepo: multiConcernRepo{
			flowRun: &FlowRunRow{ID: flowRunID, OrganizationID: orgID, Status: "running"},
			step:    newToolContractStep(stepID, flowRunID, []string{"domain_verify_start", "domain_verify_complete"}),
		},
	}
	svc := New(nil, nil, phases.NewRegistry(), "test")
	svc.Repo = repo

	res, err := svc.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID:  stepID,
		Output:         map[string]any{"scenarios_failed": []any{}},
		ToolCallsSaved: []string{"domain_verify_start"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"domain_verify_complete"}, res.MissingToolCalls,
		"debe devolver las tools faltantes")
	require.NotEqual(t, "completed", res.StepStatus,
		"el step NO debe cerrarse cuando el contrato no se cumple")
	require.Equal(t, uuid.Nil, repo.completedID,
		"MarkStepCompleted no debe llamarse (step reintentable, no failed)")
}

func TestRecordPhaseResult_ToolContract_Complete_Advances(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()

	repo := &toolContractRepo{
		multiConcernRepo: multiConcernRepo{
			flowRun:  &FlowRunRow{ID: flowRunID, OrganizationID: orgID, Status: "running"},
			step:     newToolContractStep(stepID, flowRunID, []string{"domain_verify_start", "domain_verify_complete"}),
			allSteps: []FlowRunStepRow{{ID: stepID, Status: "running", StepKey: "sdd-verify"}},
		},
	}
	svc := New(nil, nil, phases.NewRegistry(), "test")
	svc.Repo = repo

	res, err := svc.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID:  stepID,
		Output:         map[string]any{"scenarios_failed": []any{}},
		ToolCallsSaved: []string{"domain_verify_start", "domain_verify_complete", "domain_mem_save"},
	})
	require.NoError(t, err)
	require.Empty(t, res.MissingToolCalls, "contrato cumplido: nada falta")
	require.Equal(t, "completed", res.StepStatus, "el step debe cerrarse")
	require.Equal(t, stepID, repo.completedID, "MarkStepCompleted debe llamarse")
}

func TestRecordPhaseResult_ToolContract_TemplateOverride_Wins(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()

	// Default del handler vacío, pero el template exige una tool: el override gana.
	repo := &toolContractRepo{
		multiConcernRepo: multiConcernRepo{
			flowRun: &FlowRunRow{ID: flowRunID, OrganizationID: orgID, Status: "running"},
			step:    newToolContractStep(stepID, flowRunID, nil),
		},
		templateContract: []string{"domain_policy_list"},
	}
	svc := New(nil, nil, phases.NewRegistry(), "test")
	svc.Repo = repo

	res, err := svc.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID:  stepID,
		Output:         map[string]any{"scenarios_failed": []any{}},
		ToolCallsSaved: []string{},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"domain_policy_list"}, res.MissingToolCalls,
		"el contrato del template debe aplicarse aunque el handler no declare default")
}

func TestRecordPhaseResult_ToolContract_Empty_Backcompat(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()

	// Sin default y sin override: contrato vacío → cierra normal aunque el
	// cliente no reporte tool_calls (retrocompatibilidad).
	repo := &toolContractRepo{
		multiConcernRepo: multiConcernRepo{
			flowRun:  &FlowRunRow{ID: flowRunID, OrganizationID: orgID, Status: "running"},
			step:     newToolContractStep(stepID, flowRunID, nil),
			allSteps: []FlowRunStepRow{{ID: stepID, Status: "running", StepKey: "sdd-verify"}},
		},
	}
	svc := New(nil, nil, phases.NewRegistry(), "test")
	svc.Repo = repo

	res, err := svc.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID: stepID,
		Output:        map[string]any{"scenarios_failed": []any{}},
	})
	require.NoError(t, err)
	require.Empty(t, res.MissingToolCalls)
	require.Equal(t, "completed", res.StepStatus, "contrato vacío = no-op, cierra como hoy")
}
