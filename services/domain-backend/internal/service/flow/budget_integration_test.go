//go:build integration

// issue-21.6 Fase B: integration test del BudgetCache que cubre S1.3
// org_flow_config. Valida que la cache funciona con la nueva PK (id BIGSERIAL)
// y que single-org (LIMIT 1 sin organization_id) sigue sirviendo el valor.

package flow

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
)

func setupBudgetDB(t *testing.T) (*pgxpool.Pool, func()) {
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
	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestBudgetCache_DefaultIfNoConfig(t *testing.T) {
	pool, cleanup := setupBudgetDB(t)
	defer cleanup()

	cache := NewBudgetCache(pool)
	dur, err := cache.GetMaxDuration(context.Background(), uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 5*time.Minute, dur, "sin row en org_flow_config → default 5min")
}

func TestBudgetCache_ReturnsConfiguredValue(t *testing.T) {
	pool, cleanup := setupBudgetDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`INSERT INTO org_flow_config (max_flow_duration_seconds) VALUES (120)`)
	require.NoError(t, err)

	cache := NewBudgetCache(pool)
	dur, err := cache.GetMaxDuration(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 120*time.Second, dur)
}

func TestBudgetCache_InvalidateRefetches(t *testing.T) {
	pool, cleanup := setupBudgetDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`INSERT INTO org_flow_config (max_flow_duration_seconds) VALUES (60)`)
	require.NoError(t, err)

	cache := NewBudgetCache(pool)

	// Primera llamada: cache miss → fetch de DB.
	dur1, err := cache.GetMaxDuration(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 60*time.Second, dur1)

	// Cambiamos el valor en DB (simulamos operator edit).
	_, err = pool.Exec(ctx, `UPDATE org_flow_config SET max_flow_duration_seconds = 600`)
	require.NoError(t, err)

	// Sin invalidate, la cache todavía tiene el valor viejo.
	dur2, err := cache.GetMaxDuration(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 60*time.Second, dur2, "cache hit antes de invalidate")

	// Tras invalidate, refetch debe traer el nuevo.
	cache.Invalidate(uuid.Nil)
	dur3, err := cache.GetMaxDuration(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 600*time.Second, dur3, "post-invalidate trae valor nuevo")
}
