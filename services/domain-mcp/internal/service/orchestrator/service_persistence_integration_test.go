//go:build integration

package orchestrator_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

func setupOrchestratorDB(t *testing.T) (pools *db.Pools, cleanup func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pools, err = db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	return pools, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func newOrgID(t *testing.T, pools *db.Pools) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orgID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID))
	return orgID
}

func newUserID(t *testing.T, pools *db.Pools, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var userID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO users (organization_id, email)
		 VALUES ($1, 'bob@example.com') RETURNING id`,
		orgID,
	).Scan(&userID))
	return userID
}

// newProjectID crea un proyecto fixture para la org dada. Fase 2: el orquestador
// exige ProjectID (flow_runs.project_id es NOT NULL), asi que los tests que
// llaman Run necesitan un proyecto real al cual scopear la corrida.
func newProjectID(t *testing.T, pools *db.Pools, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var projectID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO projects (organization_id, name, slug)
		 VALUES ($1, 'Test Project', 'test-project') RETURNING id`,
		orgID,
	).Scan(&projectID))
	return projectID
}

func buildRegistry() *phases.Registry {
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	return reg
}

func TestService_Run_Express_WithoutSeededFlow_ReturnsErrFlowNotSeeded(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)


	s := orchestrator.New(pools.App, nil, buildRegistry(), "dev")
	_, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "fix typo",
		Mode:           orchestrator.ModeExpress,
	})
	require.ErrorIs(t, err, orchestrator.ErrFlowNotSeeded)
}

func TestService_Run_Express_PersistsFlowRunAndSteps(t *testing.T) {
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
		RawText:        "implementar typo fix",
		Mode:           orchestrator.ModeExpress,
	})
	require.NoError(t, err)
	require.Equal(t, orchestrator.ModeExpress, res.Mode)
	require.NotEqual(t, uuid.Nil, res.FlowRunID)
	require.NotNil(t, res.Plan)
	require.Len(t, res.Plan.Steps, 2)


	var (
		status      string
		flowID      uuid.UUID
		triggeredBy uuid.UUID
		metaRaw     []byte
	)
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status, flow_id, triggered_by, cursor
		   FROM flow_runs WHERE id=$1`, res.FlowRunID,
	).Scan(&status, &flowID, &triggeredBy, &metaRaw))
	require.Equal(t, "pending", status)
	require.Equal(t, userID, triggeredBy)

	var seededID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT id FROM flows WHERE slug=$1`,
		seeds.SDDPipelineFlowSlug).Scan(&seededID))
	require.Equal(t, seededID, flowID)


	var meta map[string]any
	require.NoError(t, json.Unmarshal(metaRaw, &meta))
	require.Equal(t, res.OrchestratorRunID.String(), meta["orchestrator_run_id"])
	require.Equal(t, "express", meta["mode"])
	require.Equal(t, "implementar typo fix", meta["raw_text"])


	rows, err := pools.App.Query(ctx,
		`SELECT step_key, status FROM flow_run_steps
		 WHERE flow_run_id=$1 ORDER BY created_at`, res.FlowRunID)
	require.NoError(t, err)
	var keys []string
	for rows.Next() {
		var k, st string
		require.NoError(t, rows.Scan(&k, &st))
		require.Equal(t, "pending", st)
		keys = append(keys, k)
	}
	rows.Close()
	require.Equal(t, []string{"sdd-apply", "sdd-verify"}, keys)
}

func TestService_Run_Express_StepInputsIncludeSuggestedSaves(t *testing.T) {
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

	var inputsRaw []byte
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT inputs FROM flow_run_steps
		 WHERE flow_run_id=$1 AND step_key=$2`,
		res.FlowRunID, "sdd-apply",
	).Scan(&inputsRaw))
	var inputs map[string]any
	require.NoError(t, json.Unmarshal(inputsRaw, &inputs))
	saves, ok := inputs["suggested_saves"].([]any)
	require.True(t, ok, "suggested_saves debe estar en step.inputs JSONB")
	require.Len(t, saves, 1)
	first, _ := saves[0].(map[string]any)
	require.Equal(t, "code_reference", first["type"])
	require.Equal(t, true, first["required"], "D5: code_reference Required=true en sdd-apply")
}
