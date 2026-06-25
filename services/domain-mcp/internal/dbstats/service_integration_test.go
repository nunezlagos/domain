//go:build integration

package dbstats_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/dbstats"
	dmigrate "nunezlagos/domain/internal/migrate"
)

// setupWithExtension arranca postgres con shared_preload_libraries para
// que pg_stat_statements track stats. Sin esto la extensión se puede crear
// pero no traquea nada.
func setupWithExtension(t *testing.T) (*dbstats.Service, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithCmd("postgres", "-c", "shared_preload_libraries=pg_stat_statements"),
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

	_, err = pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`)
	require.NoError(t, err)

	svc := &dbstats.Service{Pool: pool}
	return svc, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestDBStats_Available_WithExtension(t *testing.T) {
	svc, cleanup := setupWithExtension(t)
	defer cleanup()
	ok, err := svc.Available(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
}

func TestDBStats_SlowQueries_Threshold(t *testing.T) {
	svc, cleanup := setupWithExtension(t)
	defer cleanup()
	ctx := context.Background()


	for i := 0; i < 10; i++ {
		_, _ = svc.Pool.Exec(ctx, `SELECT pg_sleep(0.05)`)
	}
	queries, err := svc.SlowQueries(ctx, 40.0, 50)
	require.NoError(t, err)
	require.NotEmpty(t, queries, "queries con mean_exec_time >= 40ms deben aparecer")
	for _, q := range queries {
		require.True(t, q.MeanExecTime >= 40.0)
	}
}

func TestDBStats_Snapshot(t *testing.T) {
	svc, cleanup := setupWithExtension(t)
	defer cleanup()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _ = svc.Pool.Exec(ctx, `SELECT 1`)
	}
	result, err := svc.Snapshot(ctx)
	require.NoError(t, err)
	require.True(t, result.Inserted > 0)
	require.True(t, time.Since(result.CapturedAt) < time.Minute)


	history, err := svc.HistorySince(ctx, time.Now().Add(-1*time.Minute), 100)
	require.NoError(t, err)
	require.NotEmpty(t, history)
}

func TestDBStats_Reset(t *testing.T) {
	svc, cleanup := setupWithExtension(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = svc.Pool.Exec(ctx, `SELECT 1`)
	require.NoError(t, svc.Reset(ctx))


	queries, err := svc.SlowQueries(ctx, 0, 100)
	require.NoError(t, err)

	require.True(t, len(queries) <= 5,
		"post-reset, pg_stat_statements debe estar casi vacío")
}

// Sin shared_preload_libraries: Available() retorna false y los métodos
// devuelven ErrNotAvailable.
func TestDBStats_NotAvailable(t *testing.T) {
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
	defer pool.Close()
	defer pgC.Terminate(ctx)

	svc := &dbstats.Service{Pool: pool}
	ok, err := svc.Available(ctx)
	require.NoError(t, err)
	require.False(t, ok, "sin shared_preload_libraries la extensión no está creada")

	_, err = svc.SlowQueries(ctx, 0, 10)
	require.ErrorIs(t, err, dbstats.ErrNotAvailable)
}
