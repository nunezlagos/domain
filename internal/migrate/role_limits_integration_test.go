//go:build integration

// HU-25.8 verifica timeouts y connection limits aplicados a roles.

package migrate_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "github.com/saargo/domain/internal/migrate"
)

func setupLimits(t *testing.T) (*pgxpool.Pool, func()) {
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

	bootstrap, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	// Habilitar LOGIN + password en app_user para que las role-level GUC
	// configuradas por migration 29 (statement_timeout, lock_timeout, etc.)
	// se apliquen al login. SET ROLE NO re-aplica role-level GUCs en pg.
	_, err = bootstrap.Exec(ctx, `ALTER ROLE app_user WITH LOGIN PASSWORD 'apppass'`)
	require.NoError(t, err)
	bootstrap.Close()

	// Reconectar como app_user directamente.
	host, _ := pgC.Host(ctx)
	port, _ := pgC.MappedPort(ctx, "5432/tcp")
	appDSN := "postgres://app_user:apppass@" + host + ":" + port.Port() + "/test?sslmode=disable"
	cfg, _ := pgxpool.ParseConfig(appDSN)
	_ = cfg
	pool, err := pgxpool.New(ctx, appDSN)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// Escenario 1: app_user tiene statement_timeout = 30s.
func TestRoleLimits_AppUserStatementTimeout(t *testing.T) {
	pool, cleanup := setupLimits(t)
	defer cleanup()
	ctx := context.Background()

	var setting string
	require.NoError(t, pool.QueryRow(ctx, `SHOW statement_timeout`).Scan(&setting))
	require.Equal(t, "30s", setting, "app_user debe heredar statement_timeout=30s")
}

func TestRoleLimits_AppUserLockTimeout(t *testing.T) {
	pool, cleanup := setupLimits(t)
	defer cleanup()
	ctx := context.Background()
	var setting string
	require.NoError(t, pool.QueryRow(ctx, `SHOW lock_timeout`).Scan(&setting))
	require.Equal(t, "10s", setting)
}

func TestRoleLimits_AppUserIdleInTxTimeout(t *testing.T) {
	pool, cleanup := setupLimits(t)
	defer cleanup()
	ctx := context.Background()
	var setting string
	require.NoError(t, pool.QueryRow(ctx, `SHOW idle_in_transaction_session_timeout`).Scan(&setting))
	require.Equal(t, "1min", setting, "60s renderea como 1min en SHOW")
}

// Escenario 1 sabotaje: query lenta excede statement_timeout y aborta.
func TestSabotage_StatementTimeout_AbortsLongQuery(t *testing.T) {
	pool, cleanup := setupLimits(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Bajamos timeout a 200ms para no esperar 30s en test
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, `SET statement_timeout = '200ms'`)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `SELECT pg_sleep(2)`)
	require.Error(t, err, "pg_sleep(2) con timeout 200ms debe abortar")
	require.Contains(t, err.Error(), "canceling statement",
		"error debe ser statement_timeout, no otro")
}

// Escenario 4: connection limit configurado en pg_roles.
func TestRoleLimits_AppUserConnectionLimit(t *testing.T) {
	pool, cleanup := setupLimits(t)
	defer cleanup()
	ctx := context.Background()

	var limit int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT rolconnlimit FROM pg_roles WHERE rolname = 'app_user'`).Scan(&limit))
	require.Equal(t, 200, limit)

	var adminLimit int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT rolconnlimit FROM pg_roles WHERE rolname = 'app_admin'`).Scan(&adminLimit))
	require.Equal(t, 10, adminLimit, "app_admin cap bajo por seguridad")
}

// app_migrator no tiene timeout (migrations grandes ok).
func TestRoleLimits_AppMigratorNoStatementTimeout(t *testing.T) {
	pool, cleanup := setupLimits(t)
	defer cleanup()
	ctx := context.Background()

	rows, err := pool.Query(ctx,
		`SELECT unnest(rolconfig) FROM pg_roles WHERE rolname = 'app_migrator'`)
	require.NoError(t, err)
	defer rows.Close()
	var configs []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		configs = append(configs, s)
	}
	require.Contains(t, configs, "statement_timeout=0",
		"app_migrator debe tener statement_timeout=0 (sin límite)")
}
