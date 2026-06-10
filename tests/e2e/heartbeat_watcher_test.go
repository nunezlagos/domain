//go:build integration

// Tests integration para issue-08.11 heartbeat-watcher-cron.
package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/metrics"
	systemcron "nunezlagos/domain/internal/scheduler/cron/system"
)

// hbFixture aisla setup de heartbeat tests (no necesita services completos).
type hbFixture struct {
	pool   *pgxpool.Pool
	orgID  uuid.UUID
	flowID uuid.UUID
}

func setupHBWatcher(t *testing.T) (*hbFixture, *systemcron.HeartbeatWatcher, func()) {
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

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	// Crear org + flow base para los tests
	orgID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, 'Test Org', 'test-org')
	`, orgID)
	require.NoError(t, err)

	flowID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO flows (id, organization_id, slug, name, spec)
		VALUES ($1, $2, 'test-flow', 'Test Flow', '{}'::jsonb)
	`, flowID, orgID)
	require.NoError(t, err)

	reg := metrics.New()
	watcher := &systemcron.HeartbeatWatcher{
		Pool:    pool,
		Metrics: reg,
		Timeout: 5 * time.Minute,
		Tick:    1 * time.Second,
		Batch:   100,
	}

	cleanup := func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
	return &hbFixture{pool: pool, orgID: orgID, flowID: flowID}, watcher, cleanup
}

// Escenario 1: detecta step stuck + lo marca failed + dispara saga
func TestHeartbeatWatcher_DetectsAndMarksFailed(t *testing.T) {
	fx, watcher, cleanup := setupHBWatcher(t)
	defer cleanup()
	ctx := context.Background()

	// Insert flow_run + step running con heartbeat 6min atrás (stuck)
	runID := uuid.New()
	_, err := fx.pool.Exec(ctx, `
		INSERT INTO flow_runs (id, flow_id, organization_id, status, started_at, last_heartbeat_at)
		VALUES ($1, $2, $3, 'running', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '6 minutes')
	`, runID, fx.flowID, fx.orgID)
	require.NoError(t, err)

	stepID := uuid.New()
	_, err = fx.pool.Exec(ctx, `
		INSERT INTO flow_run_steps (id, flow_run_id, step_key, status, started_at, last_heartbeat_at)
		VALUES ($1, $2, 'sdd-design', 'running', NOW() - INTERVAL '8 minutes', NOW() - INTERVAL '6 minutes')
	`, stepID, runID)
	require.NoError(t, err)

	// Ejecutar tick
	stuck, err := watcher.DetectAndMark(ctx)
	require.NoError(t, err)
	require.Len(t, stuck, 1, "debe detectar 1 step stuck")
	require.Equal(t, stepID.String(), stuck[0].StepID)
	require.Equal(t, "sdd-design", stuck[0].StepKey)

	// Verificar step marcado failed
	var stepStatus, stepError string
	err = fx.pool.QueryRow(ctx, `SELECT status, COALESCE(error, '') FROM flow_run_steps WHERE id = $1`,
		stepID).Scan(&stepStatus, &stepError)
	require.NoError(t, err)
	require.Equal(t, "failed", stepStatus)
	require.Equal(t, "heartbeat_timeout", stepError)

	// Verificar saga_compensation_log
	var sagaCount int
	err = fx.pool.QueryRow(ctx, `SELECT COUNT(*) FROM saga_compensation_log WHERE run_id = $1`,
		runID).Scan(&sagaCount)
	require.NoError(t, err)
	require.Equal(t, 1, sagaCount, "debe haber 1 row en saga_compensation_log")

	// Verificar flow_runs.status = 'failed' (todos los steps terminados)
	var runStatus string
	err = fx.pool.QueryRow(ctx, `SELECT status FROM flow_runs WHERE id = $1`, runID).Scan(&runStatus)
	require.NoError(t, err)
	require.Equal(t, "failed", runStatus, "flow_run debe quedar failed cuando todos los steps están terminales")
}

// Escenario 2: heartbeat reciente NO se toca
func TestHeartbeatWatcher_RecentHeartbeat_NotMarked(t *testing.T) {
	fx, watcher, cleanup := setupHBWatcher(t)
	defer cleanup()
	ctx := context.Background()

	runID := uuid.New()
	_, err := fx.pool.Exec(ctx, `
		INSERT INTO flow_runs (id, flow_id, organization_id, status, started_at, last_heartbeat_at)
		VALUES ($1, $2, $3, 'running', NOW(), NOW())
	`, runID, fx.flowID, fx.orgID)
	require.NoError(t, err)

	stepID := uuid.New()
	_, err = fx.pool.Exec(ctx, `
		INSERT INTO flow_run_steps (id, flow_run_id, step_key, status, started_at, last_heartbeat_at)
		VALUES ($1, $2, 'sdd-explore', 'running', NOW(), NOW() - INTERVAL '2 minutes')
	`, stepID, runID)
	require.NoError(t, err)

	stuck, err := watcher.DetectAndMark(ctx)
	require.NoError(t, err)
	require.Empty(t, stuck, "heartbeat reciente NO debe detectarse")

	var status string
	err = fx.pool.QueryRow(ctx, `SELECT status FROM flow_run_steps WHERE id = $1`, stepID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "running", status, "step debe seguir running")
}

// Escenario 3: threshold configurable (timeout=10min, heartbeat=6min → NO stuck)
func TestHeartbeatWatcher_ConfigurableThreshold(t *testing.T) {
	fx, watcher, cleanup := setupHBWatcher(t)
	defer cleanup()
	ctx := context.Background()

	// Reconfigurar timeout a 10min
	watcher.Timeout = 10 * time.Minute

	runID := uuid.New()
	_, err := fx.pool.Exec(ctx, `
		INSERT INTO flow_runs (id, flow_id, organization_id, status, started_at, last_heartbeat_at)
		VALUES ($1, $2, $3, 'running', NOW() - INTERVAL '8 minutes', NOW() - INTERVAL '6 minutes')
	`, runID, fx.flowID, fx.orgID)
	require.NoError(t, err)

	stepID := uuid.New()
	_, err = fx.pool.Exec(ctx, `
		INSERT INTO flow_run_steps (id, flow_run_id, step_key, status, started_at, last_heartbeat_at)
		VALUES ($1, $2, 'sdd-apply', 'running', NOW() - INTERVAL '7 minutes', NOW() - INTERVAL '6 minutes')
	`, stepID, runID)
	require.NoError(t, err)

	stuck, err := watcher.DetectAndMark(ctx)
	require.NoError(t, err)
	require.Empty(t, stuck, "con timeout 10min, heartbeat 6min NO debe detectarse")
}

// Escenario 6: SABOTAJE — race condition con FOR UPDATE SKIP LOCKED
// Simulamos un cliente actualizando heartbeat con lock concurrente.
func TestHeartbeatWatcher_RaceCondition_SkipsLocked(t *testing.T) {
	fx, watcher, cleanup := setupHBWatcher(t)
	defer cleanup()
	ctx := context.Background()

	// Step stuck que vamos a "lockear" en otra tx
	runID := uuid.New()
	_, err := fx.pool.Exec(ctx, `
		INSERT INTO flow_runs (id, flow_id, organization_id, status, started_at, last_heartbeat_at)
		VALUES ($1, $2, $3, 'running', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '6 minutes')
	`, runID, fx.flowID, fx.orgID)
	require.NoError(t, err)

	stepID := uuid.New()
	_, err = fx.pool.Exec(ctx, `
		INSERT INTO flow_run_steps (id, flow_run_id, step_key, status, started_at, last_heartbeat_at)
		VALUES ($1, $2, 'sdd-tasks', 'running', NOW() - INTERVAL '8 minutes', NOW() - INTERVAL '6 minutes')
	`, stepID, runID)
	require.NoError(t, err)

	// Tomar lock explícito sobre el row en otra tx (simula cliente updating)
	otherTx, err := fx.pool.Begin(ctx)
	require.NoError(t, err)
	_, err = otherTx.Exec(ctx, `SELECT id FROM flow_run_steps WHERE id = $1 FOR UPDATE`, stepID)
	require.NoError(t, err)

	// Watcher debe skip el step lockado
	stuck, err := watcher.DetectAndMark(ctx)
	require.NoError(t, err)
	require.Empty(t, stuck, "watcher debe skip steps con lock concurrente (FOR UPDATE SKIP LOCKED)")

	// Soltar lock
	require.NoError(t, otherTx.Rollback(ctx))

	// Ahora sí lo detecta
	stuck, err = watcher.DetectAndMark(ctx)
	require.NoError(t, err)
	require.Len(t, stuck, 1, "post-rollback, watcher debe detectarlo")
}
