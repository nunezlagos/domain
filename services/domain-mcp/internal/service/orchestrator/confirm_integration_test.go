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

// D1 confirm condicional: Express + apply con 1 file y pocas líneas
// auto-avanza (sin RequiresConfirm).
func TestExpressD1_SmallChange_AutoApproves(t *testing.T) {
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
		RawText:        "fix typo",
		Mode:           orchestrator.ModeExpress,
	})
	require.NoError(t, err)

	applyID := res.Plan.Steps[0].ID
	r, err := s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: applyID,
		Output: map[string]any{
			"files_changed": []any{"a.go"},
			"lines_changed": 3,
		},
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.NoError(t, err)
	require.False(t, r.RequiresConfirm, "≤10 líneas + 1 file → auto-approve")
	require.Equal(t, "running", r.FlowRunStatus)
}

// Express con >ExpressMaxLines (default 10) → RequiresConfirm=true,
// step verify queda blocked.
func TestExpressD1_LargeChange_RequiresConfirm(t *testing.T) {
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
		RawText:        "refactor grande",
		Mode:           orchestrator.ModeExpress,
	})
	require.NoError(t, err)

	applyID := res.Plan.Steps[0].ID
	verifyID := res.Plan.Steps[1].ID

	// Reportamos apply con 25 líneas (supera el default 10)
	r, err := s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: applyID,
		Output: map[string]any{
			"files_changed": []any{"a.go"},
			"lines_changed": 25,
		},
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.NoError(t, err)
	require.True(t, r.RequiresConfirm, "25 líneas > 10 → confirm required")
	require.NotEmpty(t, r.ConfirmMessage)

	// El verify step debe estar blocked en BD
	var status string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_run_steps WHERE id=$1`, verifyID,
	).Scan(&status))
	require.Equal(t, "blocked", status)

	// ConfirmContinue(true) lo desbloquea
	confRes, err := s.ConfirmContinue(ctx, res.FlowRunID, true)
	require.NoError(t, err)
	require.Equal(t, "pending", confRes.StepStatus)
	require.Equal(t, verifyID, confRes.StepID)
	require.NotEmpty(t, confRes.NextStepPrompt)

	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_run_steps WHERE id=$1`, verifyID,
	).Scan(&status))
	require.Equal(t, "pending", status)
}

// Multi-file también dispara confirm (RFC 0006 D1: "≤10 líneas + single-file").
func TestExpressD1_MultiFile_RequiresConfirm(t *testing.T) {
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

	applyID := res.Plan.Steps[0].ID
	r, err := s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: applyID,
		Output: map[string]any{
			"files_changed": []any{"a.go", "b.go"},
			"lines_changed": 5,
		},
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.NoError(t, err)
	require.True(t, r.RequiresConfirm, "multi-file → confirm sin importar líneas")
}

// ConfirmContinue(false) rechaza y marca el flow como failed.
func TestExpressD1_RejectConfirm_MarksFlowFailed(t *testing.T) {
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
		OrganizationID:  orgID,
		ProjectID:      projectID,
		UserID:          userID,
		RawText:         "x",
		Mode:            orchestrator.ModeExpress,
		ExpressMaxLines: 5, // override threshold
	})
	require.NoError(t, err)

	applyID := res.Plan.Steps[0].ID
	_, err = s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: applyID,
		Output: map[string]any{
			"files_changed": []any{"a.go"},
			"lines_changed": 20,
		},
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.NoError(t, err)

	// Usuario rechaza
	confRes, err := s.ConfirmContinue(ctx, res.FlowRunID, false)
	require.NoError(t, err)
	require.Equal(t, "failed", confRes.StepStatus)
	require.Equal(t, "failed", confRes.FlowRunStatus)

	st, err := s.GetFlowStatus(ctx, res.FlowRunID)
	require.NoError(t, err)
	require.Equal(t, "failed", st.Status)
}
