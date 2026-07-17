//go:build integration

package txctx_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/store/txctx"
)

// setupRLS levanta una Postgres migrada con el rol app_user activo.
func setupRLS(t *testing.T) (*pgxpool.Pool, func()) {
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

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)

	bootstrap, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	_, err = bootstrap.Exec(ctx, `GRANT app_user TO test`)
	require.NoError(t, err)
	bootstrap.Close()
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE app_user`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestWithOrgTx_RejectsNilUUID(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	err := txctx.WithOrgTx(context.Background(), pool, uuid.Nil, func(pgx.Tx) error {
		return nil
	})
	require.Error(t, err)
}
