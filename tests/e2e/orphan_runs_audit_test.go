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

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/metrics"
	systemcron "nunezlagos/domain/internal/scheduler/cron/system"
)

type orphanFixture struct {
	pool    *pgxpool.Pool
	orgID   uuid.UUID
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

	orgID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, 'Test Org', 'test-org')
	`, orgID)
	require.NoError(t, err)

	// agente base para crear agent_runs
	agentID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO agents (id, organization_id, slug, name, system_prompt, model, provider)
		VALUES ($1, $2, 'test-agent', 'Test', 'sys', 'claude-haiku-4-5-20251001', 'anthropic')
	`, agentID, orgID)
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
	return &orphanFixture{pool: pool, orgID: orgID, agentID: agentID}, auditor, cleanup
}

// Escenario 1: detección bypass (flow_run_id NULL + sin standalone)
func TestOrphanAudit_DetectsBypass(t *testing.T) {
	fx, auditor, cleanup := setupOrphanAuditor(t)
	defer cleanup()
	ctx := context.Background()

	// Insertar 3 agent_runs orphan (sin flow_run_id, sin metadata.standalone)
	for i := 0; i < 3; i++ {
		_, err := fx.pool.Exec(ctx, `
			INSERT INTO agent_runs (id, organization_id, agent_id, flow_run_id, status, metadata)
			VALUES (gen_random_uuid(), $1, $2, NULL, 'completed', '{}'::jsonb)
		`, fx.orgID, fx.agentID)
		require.NoError(t, err)
	}

	rows, _, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "debe haber 1 org agregada")
	require.Equal(t, fx.orgID.String(), rows[0].OrgID)
	require.Equal(t, int64(3), rows[0].Count)
}

// Escenario 2: standalone=true NO se cuenta
func TestOrphanAudit_StandaloneIgnored(t *testing.T) {
	fx, auditor, cleanup := setupOrphanAuditor(t)
	defer cleanup()
	ctx := context.Background()

	// 2 con standalone=true (legítimos) + 1 sin (bypass)
	_, err := fx.pool.Exec(ctx, `
		INSERT INTO agent_runs (id, organization_id, agent_id, flow_run_id, status, metadata)
		VALUES
		  (gen_random_uuid(), $1, $2, NULL, 'completed', '{"standalone":true,"reason":"debug"}'::jsonb),
		  (gen_random_uuid(), $1, $2, NULL, 'completed', '{"standalone":true,"reason":"test"}'::jsonb),
		  (gen_random_uuid(), $1, $2, NULL, 'completed', '{}'::jsonb)
	`, fx.orgID, fx.agentID)
	require.NoError(t, err)

	rows, _, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "debe haber 1 org en el agregado")
	require.Equal(t, int64(1), rows[0].Count, "sólo el 'bypass' (1) debe contarse")
}

// Escenario 3: idempotencia via last_ack_at
func TestOrphanAudit_IdempotentLastAck(t *testing.T) {
	fx, auditor, cleanup := setupOrphanAuditor(t)
	defer cleanup()
	ctx := context.Background()

	// Insertar 2 orphans
	for i := 0; i < 2; i++ {
		_, err := fx.pool.Exec(ctx, `
			INSERT INTO agent_runs (id, organization_id, agent_id, flow_run_id, status, metadata)
			VALUES (gen_random_uuid(), $1, $2, NULL, 'completed', '{}'::jsonb)
		`, fx.orgID, fx.agentID)
		require.NoError(t, err)
	}

	// Primera pasada vía runTick (avanza last_ack)
	rows, lastSeen, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), rows[0].Count)
	require.False(t, lastSeen.IsZero())

	// Simular avance de last_ack persistido (lo hace runTick internamente; acá lo hacemos manual via re-exec same logic)
	// La 2da pasada con lastSeen != NULL debería retornar 0 orphans porque fueron contados
	_, err = fx.pool.Exec(ctx, `
		INSERT INTO system_state (key, value, updated_at)
		VALUES ('orphan_runs_audit', $1::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
	`, `{"last_ack_at":"`+lastSeen.UTC().Format(time.RFC3339Nano)+`"}`)
	require.NoError(t, err)

	rows2, _, err := auditor.Audit(ctx)
	require.NoError(t, err)
	totalCount := int64(0)
	for _, r := range rows2 {
		totalCount += r.Count
	}
	require.Equal(t, int64(0), totalCount, "los orphans previos NO deben double-counted")
}

// Sabotage: bypaseo intencional → cron lo detecta
func TestOrphanAudit_Sabotage_BypassDetected(t *testing.T) {
	fx, auditor, cleanup := setupOrphanAuditor(t)
	defer cleanup()
	ctx := context.Background()

	// Sabotage: INSERT directo, simulating someone bypassing service layer
	_, err := fx.pool.Exec(ctx, `
		INSERT INTO agent_runs (id, organization_id, agent_id, flow_run_id, status, metadata)
		VALUES (gen_random_uuid(), $1, $2, NULL, 'running', '{}'::jsonb)
	`, fx.orgID, fx.agentID)
	require.NoError(t, err)

	rows, _, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "el bypass debe ser detectado")
	require.Equal(t, int64(1), rows[0].Count)
}
