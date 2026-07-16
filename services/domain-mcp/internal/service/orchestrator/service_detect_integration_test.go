//go:build integration

package orchestrator_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/orchestrator"
)

// Detect = dry-run: plan completo (12 fases) hidratado pero sin
// flow_run/steps persistidos en BD. Si el caller quiere ejecutar de
// verdad, vuelve a invocar con Mode=Full.
func TestService_Run_Detect_BuildsPlanWithoutPersistence(t *testing.T) {
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

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "preview de feature X",
		Mode:           orchestrator.ModeDetect,
	})
	require.NoError(t, err)
	require.Equal(t, orchestrator.ModeDetect, res.Mode)
	require.Len(t, res.Plan.Steps, 12, "Detect arma plan completo igual que Full")
	require.NotEmpty(t, res.Plan.Steps[0].UserPrompt, "primer prompt construido")
	require.NotEmpty(t, res.Plan.Steps[0].SystemPrompt, "system_prompt hidratado desde BD")

	var count int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM flow_runs WHERE id=$1`, res.FlowRunID,
	).Scan(&count))
	require.Equal(t, 0, count, "Detect NO debe persistir flow_run")

	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM flow_run_steps WHERE flow_run_id=$1`, res.FlowRunID,
	).Scan(&count))
	require.Equal(t, 0, count, "Detect NO debe persistir flow_run_steps")
}

// Sin flow seedeado, Detect debe fallar igual que Full — devolver
// preview de algo que la org no puede ejecutar sería confuso.
func TestService_Run_Detect_RequiresSeededFlow(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	_, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "x",
		Mode:           orchestrator.ModeDetect,
	})
	require.ErrorIs(t, err, orchestrator.ErrFlowNotSeeded)
}
