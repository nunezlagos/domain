//go:build integration





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


	dur1, err := cache.GetMaxDuration(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 60*time.Second, dur1)


	_, err = pool.Exec(ctx, `UPDATE org_flow_config SET max_flow_duration_seconds = 600`)
	require.NoError(t, err)


	dur2, err := cache.GetMaxDuration(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 60*time.Second, dur2, "cache hit antes de invalidate")


	cache.Invalidate(uuid.Nil)
	dur3, err := cache.GetMaxDuration(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 600*time.Second, dur3, "post-invalidate trae valor nuevo")
}
