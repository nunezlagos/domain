//go:build integration

package orchestrator_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// TestExpress_FullHappyPath simula el flujo completo Express end-to-end:
//   1. orchestrate → flow_run con 2 steps pending
//   2. RecordPhaseResult sdd-apply CON code_reference → step completed,
//      flow_run sigue running, next step = sdd-verify
//   3. RecordPhaseResult sdd-verify → step completed, flow_run completed
//   4. GetFlowStatus refleja todo terminal
func TestExpress_FullHappyPath(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildRegistry(), "dev")

	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "fix typo en CHANGELOG.md",
		Mode:           orchestrator.ModeExpress,
	})
	require.NoError(t, err)
	require.Len(t, res.Plan.Steps, 2)

	applyStepID := res.Plan.Steps[0].ID
	verifyStepID := res.Plan.Steps[1].ID


	appRes, err := s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: applyStepID,
		Output: map[string]any{
			"files_changed": []any{"CHANGELOG.md"},
			"summary":       "typo fix",
		},
		MemoryRefsSaved: []phases.MemoryRef{
			{Type: "code_reference", ID: uuid.New()},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "completed", appRes.StepStatus)
	require.Equal(t, "running", appRes.FlowRunStatus)
	require.NotNil(t, appRes.NextStepID)
	require.Equal(t, verifyStepID, *appRes.NextStepID)
	require.Equal(t, "sdd-verify", appRes.NextStepKey)
	require.NotEmpty(t, appRes.NextStepPrompt)


	verRes, err := s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: verifyStepID,
		Output: map[string]any{
			"scenarios_failed": []any{},
			"tests_passed":     1,
			"tests_failed":     0,
		},
		// sdd-verify declara un contrato required_tool_calls (domain_verify_*);
		// sin reportarlas el step queda running (reintentable), no completed.
		ToolCallsSaved: []string{"domain_verify_start", "domain_verify_complete"},
	})
	require.NoError(t, err)
	require.Equal(t, "completed", verRes.StepStatus)
	require.Equal(t, "completed", verRes.FlowRunStatus)
	require.Nil(t, verRes.NextStepID)


	st, err := s.GetFlowStatus(ctx, res.FlowRunID)
	require.NoError(t, err)
	require.Equal(t, "completed", st.Status)
	require.Equal(t, "express", st.Mode)
	require.Equal(t, res.OrchestratorRunID.String(), st.OrchestratorRunID)
	require.Len(t, st.Steps, 2)
	for _, s := range st.Steps {
		require.Equal(t, "completed", s.Status)
	}
}

// code_reference dejó de ser un required save en sdd-apply tras el retiro del
// code graph (2026-07-07): exigirlo mataba todo sdd-apply (ver sdd_apply.go).
// Este test fija esa verdad de prod — reportar apply SIN code_reference ya NO
// bloquea: el step completa, no hay missing_required_saves, y el flow avanza a
// sdd-verify. Si alguien vuelve a marcar code_reference como Required=true, este
// test se rompe y fuerza revisión consciente del contrato.
func TestExpress_ApplyWithoutCodeReference_Completes(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildRegistry(), "dev")
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "x",
		Mode:           orchestrator.ModeExpress,
	})
	require.NoError(t, err)

	applyStepID := res.Plan.Steps[0].ID
	appRes, err := s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: applyStepID,
		Output:        map[string]any{"summary": "looks good"},
	})
	require.NoError(t, err)
	require.Equal(t, "completed", appRes.StepStatus)
	require.Empty(t, appRes.MissingRequiredSaves, "code_reference es opcional: no debe faltar nada")
	require.Equal(t, "running", appRes.FlowRunStatus)
	require.Equal(t, "sdd-verify", appRes.NextStepKey)

	st, err := s.GetFlowStatus(ctx, res.FlowRunID)
	require.NoError(t, err)
	require.Equal(t, "running", st.Status, "flow sigue running: apply completó sin required save")
	require.Equal(t, "completed", st.Steps[0].Status)
}

// El cliente reporta sobre un step ya completado → debe fallar con
// ErrFlowRunStepNotPending (no se permite re-marcar steps terminales).
func TestExpress_PhaseResult_OnAlreadyCompletedStep_Rejected(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildRegistry(), "dev")
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "x",
		Mode:           orchestrator.ModeExpress,
	})
	require.NoError(t, err)
	applyStepID := res.Plan.Steps[0].ID


	_, err = s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID:   applyStepID,
		Output:          map[string]any{"summary": "ok"},
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.NoError(t, err)


	_, err = s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID:   applyStepID,
		Output:          map[string]any{"summary": "retry"},
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.ErrorIs(t, err, orchestrator.ErrFlowRunStepNotPending)
}

// GetFlowStatus de un flow_run_id que no existe debe ser ErrFlowRunNotFound.
func TestGetFlowStatus_UnknownID_NotFound(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	s := orchestrator.New(pools.App, nil, buildRegistry(), "dev")
	_, err := s.GetFlowStatus(ctx, uuid.New())
	require.ErrorIs(t, err, orchestrator.ErrFlowRunNotFound)
}
