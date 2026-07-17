//go:build integration

// Tests integration para issue-08.12 orphan-runs-audit-cron.
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

	"nunezlagos/domain/internal/metrics"
	dmigrate "nunezlagos/domain/internal/migrate"
	systemcron "nunezlagos/domain/internal/scheduler/cron/system"
)

type orphanFixture struct {
	pool    *pgxpool.Pool
	agentID uuid.UUID
}

func setupOrphanAuditor(t *testing.T) (*orphanFixture, *systemcron.OrphanAuditor, func()) {
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

	agentID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO agents (id, slug, name, system_prompt, model, provider)
		VALUES ($1, 'test-agent', 'Test', 'sys', 'claude-haiku-4-5-20251001', 'anthropic')
	`, agentID)
	require.NoError(t, err)

	reg := metrics.New()
	auditor := &systemcron.OrphanAuditor{
		Pool:    pool,
		Metrics: reg,
		Tick:    1 * time.Hour,
		Batch:   1000,
	}

	cleanup := func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
	return &orphanFixture{pool: pool, agentID: agentID}, auditor, cleanup
}

// Escenario 1: detección bypass (flow_run_id NULL + sin standalone)
func TestOrphanAudit_DetectsBypass(t *testing.T) {
	fx, auditor, cleanup := setupOrphanAuditor(t)
	defer cleanup()
	ctx := context.Background()


	for i := 0; i < 3; i++ {
		_, err := fx.pool.Exec(ctx, `
			INSERT INTO agent_runs (id, agent_id, flow_run_id, status, metadata)
			VALUES (gen_random_uuid(), $1, NULL, 'completed', '{}'::jsonb)
		`, fx.agentID)
		require.NoError(t, err)
	}

	rows, _, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "debe haber 1 fila agregada")
	require.Equal(t, int64(3), rows[0].Count)
}

// Escenario 2: standalone=true NO se cuenta
func TestOrphanAudit_StandaloneIgnored(t *testing.T) {
	fx, auditor, cleanup := setupOrphanAuditor(t)
	defer cleanup()
	ctx := context.Background()


	_, err := fx.pool.Exec(ctx, `
		INSERT INTO agent_runs (id, agent_id, flow_run_id, status, metadata)
		VALUES
		  (gen_random_uuid(), $1, NULL, 'completed', '{"standalone":true,"reason":"debug"}'::jsonb),
		  (gen_random_uuid(), $1, NULL, 'completed', '{"standalone":true,"reason":"test"}'::jsonb),
		  (gen_random_uuid(), $1, NULL, 'completed', '{}'::jsonb)
	`, fx.agentID)
	require.NoError(t, err)

	rows, _, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "debe haber 1 org en el agregado")
	require.Equal(t, int64(1), rows[0].Count, "sólo el 'bypass' (1) debe contarse")
}

// Escenario 3: la auditoría agrega el conteo de orphans en la ventana.
// REQ-42.3: el cursor last_ack_at ya NO se persiste en system_state (tabla
// dropeada); vive en memoria y lo avanza runTick. Este test verifica que una
// pasada de Audit cuenta los orphans existentes en la ventana.
func TestOrphanAudit_CountsOrphansInWindow(t *testing.T) {
	fx, auditor, cleanup := setupOrphanAuditor(t)
	defer cleanup()
	ctx := context.Background()


	for i := 0; i < 2; i++ {
		_, err := fx.pool.Exec(ctx, `
			INSERT INTO agent_runs (id, agent_id, flow_run_id, status, metadata)
			VALUES (gen_random_uuid(), $1, NULL, 'completed', '{}'::jsonb)
		`, fx.agentID)
		require.NoError(t, err)
	}

	rows, lastSeen, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), rows[0].Count)
	require.False(t, lastSeen.IsZero())
}

// Sabotage: bypaseo intencional → cron lo detecta
func TestOrphanAudit_Sabotage_BypassDetected(t *testing.T) {
	fx, auditor, cleanup := setupOrphanAuditor(t)
	defer cleanup()
	ctx := context.Background()


	_, err := fx.pool.Exec(ctx, `
		INSERT INTO agent_runs (id, agent_id, flow_run_id, status, metadata)
		VALUES (gen_random_uuid(), $1, NULL, 'running', '{}'::jsonb)
	`, fx.agentID)
	require.NoError(t, err)

	rows, _, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "el bypass debe ser detectado")
	require.Equal(t, int64(1), rows[0].Count)
}
