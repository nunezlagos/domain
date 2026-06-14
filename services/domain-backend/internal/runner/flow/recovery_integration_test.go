//go:build integration

package flowrunner_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/observation"
	orgsvc "nunezlagos/domain/internal/service/org"
	projsvc "nunezlagos/domain/internal/service/project"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

func recoverySetup(t *testing.T) (*flowrunner.Runner, *flow.Service, *skillsvc.Service, *projsvc.Service, uuid.UUID, uuid.UUID, uuid.UUID, func()) {
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

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	rec := &audit.NopRecorder{}
	orgS := &orgsvc.Service{Pool: pools.Auth, Audit: rec}
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}
	flowS := &flow.Service{Pool: pools.App, Audit: rec}
	// NopEmbedder evita llamadas LLM reales en tests de recovery
	skillS := &skillsvc.Service{Pool: pools.App, Audit: rec, Embedder: llm.NopEmbedder{}}
	obsS := &observation.Service{Pool: pools.App, Audit: rec, Embedder: llm.NopEmbedder{}}

	org, owner, _ := orgS.Create(ctx, "RecoveryTestOrg", "recoverytest", "r@x.com", "R")
	proj, _ := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "RecoveryProj", Slug: "rproj", ActorID: owner.UserID,
	})

	runner := &flowrunner.Runner{
		Pool: pools.App, Audit: rec, Flows: flowS,
		Skills: skillS, Observations: obsS,
		SkillRunner: skillrunner.New(),
	}
	return runner, flowS, skillS, projS, org.ID, proj.ID, owner.UserID, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

// TestRecovery_ReleaseStaleRun verifica que releaseStaleRuns libera un
// flow_run cuyo heartbeat expiró (simula crash del worker).
func TestRecovery_ReleaseStaleRun(t *testing.T) {
	runner, flowS, skillS, _, orgID, _, userID, cleanup := recoverySetup(t)
	defer cleanup()
	ctx := context.Background()

	// Crear un skill simple para el flow
	_, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "t1", Name: "T1",
		SkillType: skillsvc.TypePrompt, Content: "ok",
		InputSchema: map[string]any{"type": "object"},
		ActorID:    userID,
	})
	require.NoError(t, err)

	fl, err := flowS.Create(ctx, flow.CreateInput{
		OrganizationID: orgID, Slug: "recovery-flow", Name: "Recovery Flow",
		Spec: flow.Spec{
			Version: 1,
			Steps: []flow.Step{
				{ID: "s1", Type: flow.StepTypeSkillRun,
					Config: map[string]any{"skill_slug": "t1", "args": map[string]any{}}},
			},
		},
		ActorID: userID,
	})
	require.NoError(t, err)

	// Crear un flow_run directamente en estado 'running' con heartbeat viejo
	var runID uuid.UUID
	err = runner.Pool.QueryRow(ctx, `
		INSERT INTO flow_runs (organization_id, flow_id, triggered_by, status, started_at, last_heartbeat_at, worker_id)
		VALUES ($1, $2, $3, 'running', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '10 minutes', 'stale-worker-1')
		RETURNING id`, orgID, fl.ID, &userID).Scan(&runID)
	require.NoError(t, err)

	// Ejecutar recovery
	released, failed, err := runner.ReleaseStaleRuns(ctx, 1*time.Minute, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), released, "should release 1 stale run")
	require.Equal(t, int64(0), failed, "should not fail any runs")

	// Verificar que el worker_id se limpió
	var workerID *string
	err = runner.Pool.QueryRow(ctx, `SELECT worker_id FROM flow_runs WHERE id = $1`, runID).Scan(&workerID)
	require.NoError(t, err)
	require.Nil(t, workerID, "worker_id should be NULL after release")
}

// TestRecovery_CrashLoopDetection verifica que runs con muchas recuperaciones
// se marcan como failed.
func TestRecovery_CrashLoopDetection(t *testing.T) {
	runner, flowS, skillS, _, orgID, _, userID, cleanup := recoverySetup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "t2", Name: "T2",
		SkillType: skillsvc.TypePrompt, Content: "ok",
		InputSchema: map[string]any{"type": "object"},
		ActorID:    userID,
	})
	require.NoError(t, err)

	fl, err := flowS.Create(ctx, flow.CreateInput{
		OrganizationID: orgID, Slug: "crash-loop", Name: "Crash Loop",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "s1", Type: flow.StepTypeSkillRun, Config: map[string]any{"skill_slug": "t2", "args": map[string]any{}}},
		}},
		ActorID: userID,
	})
	require.NoError(t, err)

	// Crear run con recovery_count alto + heartbeat viejo
	var runID uuid.UUID
	err = runner.Pool.QueryRow(ctx, `
		INSERT INTO flow_runs (organization_id, flow_id, triggered_by, status, recovery_count, started_at, last_heartbeat_at, worker_id)
		VALUES ($1, $2, $3, 'running', 10, NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '10 minutes', 'stale-worker-2')
		RETURNING id`, orgID, fl.ID, &userID).Scan(&runID)
	require.NoError(t, err)

	// Recovery con maxRecoveries=5 → debe marcar como failed
	released, failed, err := runner.ReleaseStaleRuns(ctx, 1*time.Minute, 5)
	require.NoError(t, err)
	require.Equal(t, int64(0), released, "should not release crash-loop runs")
	require.Equal(t, int64(1), failed, "should fail crash-loop run")

	var status string
	err = runner.Pool.QueryRow(ctx, `SELECT status FROM flow_runs WHERE id = $1`, runID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "failed", status, "crash-loop run should be failed")
}

// TestClaimRun_Pending verifica que ClaimRun toma un run pending.
func TestClaimRun_Pending(t *testing.T) {
	runner, flowS, skillS, _, orgID, _, userID, cleanup := recoverySetup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "t3", Name: "T3",
		SkillType: skillsvc.TypePrompt, Content: "ok",
		InputSchema: map[string]any{"type": "object"},
		ActorID:    userID,
	})
	require.NoError(t, err)

	fl, err := flowS.Create(ctx, flow.CreateInput{
		OrganizationID: orgID, Slug: "claim-test", Name: "Claim Test",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "s1", Type: flow.StepTypeSkillRun, Config: map[string]any{"skill_slug": "t3", "args": map[string]any{}}},
		}},
		ActorID: userID,
	})
	require.NoError(t, err)

	// Crear run pending
	var runID uuid.UUID
	err = runner.Pool.QueryRow(ctx, `
		INSERT INTO flow_runs (organization_id, flow_id, triggered_by, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id`, orgID, fl.ID, &userID).Scan(&runID)
	require.NoError(t, err)

	claimer := &flowrunner.ClaimRunClaims{
		Pool:       runner.Pool,
		WorkerID:   "test-worker-1",
		StaleAfter: 5 * time.Minute,
	}
	claimed, err := claimer.ClaimRun(ctx)
	require.NoError(t, err)
	require.NotNil(t, claimed, "should claim pending run")
	require.Equal(t, runID, claimed.RunID)
	require.False(t, claimed.IsRecovery, "pending claim should not be recovery")
}

// TestClaimRun_Stale verifica que ClaimRun toma un run con heartbeat expirado.
func TestClaimRun_Stale(t *testing.T) {
	runner, flowS, skillS, _, orgID, _, userID, cleanup := recoverySetup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "t4", Name: "T4",
		SkillType: skillsvc.TypePrompt, Content: "ok",
		InputSchema: map[string]any{"type": "object"},
		ActorID:    userID,
	})
	require.NoError(t, err)

	fl, err := flowS.Create(ctx, flow.CreateInput{
		OrganizationID: orgID, Slug: "stale-claim", Name: "Stale Claim",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "s1", Type: flow.StepTypeSkillRun, Config: map[string]any{"skill_slug": "t4", "args": map[string]any{}}},
		}},
		ActorID: userID,
	})
	require.NoError(t, err)

	var runID uuid.UUID
	err = runner.Pool.QueryRow(ctx, `
		INSERT INTO flow_runs (organization_id, flow_id, triggered_by, status, last_heartbeat_at, worker_id)
		VALUES ($1, $2, $3, 'running', NOW() - INTERVAL '10 minutes', 'dead-worker')
		RETURNING id`, orgID, fl.ID, &userID).Scan(&runID)
	require.NoError(t, err)

	claimer := &flowrunner.ClaimRunClaims{
		Pool:       runner.Pool,
		WorkerID:   "test-worker-2",
		StaleAfter: 1 * time.Minute,
	}
	claimed, err := claimer.ClaimRun(ctx)
	require.NoError(t, err)
	require.NotNil(t, claimed, "should claim stale run")
	require.Equal(t, runID, claimed.RunID)
	require.True(t, claimed.IsRecovery, "stale claim should be recovery")
}

// TestRace_TwoWorkersClaim verifica que dos workers no pueden claimear el mismo run.
func TestRace_TwoWorkersClaim(t *testing.T) {
	runner, flowS, skillS, _, orgID, _, userID, cleanup := recoverySetup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "t5", Name: "T5",
		SkillType: skillsvc.TypePrompt, Content: "ok",
		InputSchema: map[string]any{"type": "object"},
		ActorID:    userID,
	})
	require.NoError(t, err)

	fl, err := flowS.Create(ctx, flow.CreateInput{
		OrganizationID: orgID, Slug: "race-test", Name: "Race Test",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "s1", Type: flow.StepTypeSkillRun, Config: map[string]any{"skill_slug": "t5", "args": map[string]any{}}},
		}},
		ActorID: userID,
	})
	require.NoError(t, err)

	// Solo 1 run pending
	var runID uuid.UUID
	err = runner.Pool.QueryRow(ctx, `
		INSERT INTO flow_runs (organization_id, flow_id, triggered_by, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id`, orgID, fl.ID, &userID).Scan(&runID)
	require.NoError(t, err)

	// Dos workers claimean concurrentemente
	claimer1 := &flowrunner.ClaimRunClaims{Pool: runner.Pool, WorkerID: "race-w1", StaleAfter: 5 * time.Minute}
	claimer2 := &flowrunner.ClaimRunClaims{Pool: runner.Pool, WorkerID: "race-w2", StaleAfter: 5 * time.Minute}

	var claimed1, claimed2 *flowrunner.ClaimedRun
	done := make(chan struct{}, 2)

	go func() {
		var err error
		claimed1, err = claimer1.ClaimRun(ctx)
		require.NoError(t, err)
		done <- struct{}{}
	}()
	go func() {
		var err error
		claimed2, err = claimer2.ClaimRun(ctx)
		require.NoError(t, err)
		done <- struct{}{}
	}()

	<-done
	<-done

	// Solo uno debe haber obtenido el claim
	claims := 0
	if claimed1 != nil {
		claims++
	}
	if claimed2 != nil {
		claims++
	}
	require.Equal(t, 1, claims, "only one worker should claim the run")
}
