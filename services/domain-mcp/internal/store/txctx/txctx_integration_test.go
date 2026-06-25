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

func seedTwoOrgs(t *testing.T, pool *pgxpool.Pool) (orgA, orgB uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('A', 'a') RETURNING id`).Scan(&orgA))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('B', 'b') RETURNING id`).Scan(&orgB))
	return
}

func TestWithOrgTx_RejectsNilUUID(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	err := txctx.WithOrgTx(context.Background(), pool, uuid.Nil, func(pgx.Tx) error {
		return nil
	})
	require.Error(t, err)
}

// Escenario 1: WithOrgTx setea contexto + queries devuelven solo rows de la org.
func TestRLS_Secrets_OrgIsolation(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, orgB := seedTwoOrgs(t, pool)


	err := txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO auth_secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'apikey', 'OpenAI key', '\x00')`, orgA)
		return err
	})
	require.NoError(t, err)


	err = txctx.WithOrgTx(ctx, pool, orgB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO auth_secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'apikey', 'OpenAI key', '\x00')`, orgB)
		return err
	})
	require.NoError(t, err)


	var countA int
	err = txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM auth_secrets`).Scan(&countA)
	})
	require.NoError(t, err)
	require.Equal(t, 1, countA, "org A debe ver solo SU secret")


	var countB int
	err = txctx.WithOrgTx(ctx, pool, orgB, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM auth_secrets`).Scan(&countB)
	})
	require.NoError(t, err)
	require.Equal(t, 1, countB, "org B debe ver solo SU secret")
}

// Sabotaje (issue-25.5 escenario 1): query sin SET LOCAL → 0 rows.
// Esta es la prueba CLAVE: defense-in-depth contra bugs RBAC.
func TestSabotage_RLS_NoSetLocal_ZeroRows(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, _ := seedTwoOrgs(t, pool)


	err := txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO auth_secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'k1', 'v1', '\x00')`, orgA)
		return err
	})
	require.NoError(t, err)


	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM auth_secrets`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "query sin SET LOCAL DEBE devolver 0 rows")
}

// Sabotaje: cross-org leak intentional (bug RBAC simulado).
// App intenta SELECT con org A pero pasando ID de secret de org B → no encuentra.
func TestSabotage_RLS_CrossOrgLeak_Blocked(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, orgB := seedTwoOrgs(t, pool)

	var secretIDB uuid.UUID
	err := txctx.WithOrgTx(ctx, pool, orgB, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO auth_secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'secret-b', 'B', '\x00') RETURNING id`, orgB).Scan(&secretIDB)
	})
	require.NoError(t, err)


	var found int
	err = txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM auth_secrets WHERE id = $1`, secretIDB).Scan(&found)
	})
	require.NoError(t, err)
	require.Equal(t, 0, found, "cross-org SELECT con id explícito DEBE devolver 0")
}

// Sabotaje: WITH CHECK previene INSERT en org incorrecta.
func TestSabotage_RLS_InsertWrongOrg_Rejected(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, orgB := seedTwoOrgs(t, pool)


	err := txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO auth_secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'leak', 'L', '\x00')`, orgB)
		return err
	})
	require.Error(t, err, "INSERT con org incorrecta debe rechazar por WITH CHECK")
}

// Escenario: activity_log también respeta RLS.
func TestRLS_ActivityLog_OrgIsolation(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, orgB := seedTwoOrgs(t, pool)

	for _, org := range []uuid.UUID{orgA, orgB} {
		err := txctx.WithOrgTx(ctx, pool, org, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`INSERT INTO audit_activity_log (organization_id, action, entity_type, summary)
				 VALUES ($1, 'test', 'x', 'evento')`, org)
			return err
		})
		require.NoError(t, err)
	}

	var countA int
	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM audit_activity_log`).Scan(&countA)
	}))
	require.Equal(t, 1, countA)
}

// Escenario: auth_api_keys también scoped.
func TestRLS_APIKeys_OrgIsolation(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, orgB := seedTwoOrgs(t, pool)


	makeUser := func(org uuid.UUID, email string) uuid.UUID {
		var uid uuid.UUID
		err := pool.QueryRow(ctx,
			`INSERT INTO users (organization_id, email) VALUES ($1, $2) RETURNING id`,
			org, email).Scan(&uid)
		require.NoError(t, err)
		return uid
	}
	uidA := makeUser(orgA, "alice@a.com")
	uidB := makeUser(orgB, "bob@b.com")

	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO auth_api_keys (organization_id, user_id, key_hash, key_prefix, name)
			 VALUES ($1, $2, '\x00', 'domk_live_aaaaaaa', 'A1')`, orgA, uidA)
		return err
	}))
	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO auth_api_keys (organization_id, user_id, key_hash, key_prefix, name)
			 VALUES ($1, $2, '\x00', 'domk_live_bbbbbbb', 'B1')`, orgB, uidB)
		return err
	}))

	var countA int
	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM auth_api_keys`).Scan(&countA)
	}))
	require.Equal(t, 1, countA, "org A ve solo SU api_key")
}

// SET LOCAL muere al COMMIT (importante para pgbouncer transaction-pool).
func TestRLS_SetLocalScopedToTx(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, _ := seedTwoOrgs(t, pool)


	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO auth_secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'k', 'v', '\x00')`, orgA)
		return err
	}))


	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM auth_secrets`).Scan(&count))
	require.Equal(t, 0, count, "post-tx sin SET LOCAL: RLS deniega")
}
