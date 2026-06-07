//go:build integration

// HU-25.5 RLS integration tests con testcontainers.
// Cubre defense-in-depth: queries SIN SET LOCAL deben devolver 0 rows.

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

	dmigrate "github.com/saargo/domain/internal/migrate"
	"github.com/saargo/domain/internal/store/txctx"
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

	// Hacer que cada connection del pool corra como app_user (NOBYPASSRLS).
	// Sin esto el user test bypassa RLS (es DB owner) y los tests darían falsos verdes.
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	// GRANT app_user TO test debe correr una vez, antes del AfterConnect.
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

	// Insert secret A en org A
	err := txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'apikey', 'OpenAI key', '\x00')`, orgA)
		return err
	})
	require.NoError(t, err)

	// Insert secret B en org B
	err = txctx.WithOrgTx(ctx, pool, orgB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'apikey', 'OpenAI key', '\x00')`, orgB)
		return err
	})
	require.NoError(t, err)

	// Query con context A: ve solo 1 (la suya)
	var countA int
	err = txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM secrets`).Scan(&countA)
	})
	require.NoError(t, err)
	require.Equal(t, 1, countA, "org A debe ver solo SU secret")

	// Query con context B: ve solo 1 (la suya)
	var countB int
	err = txctx.WithOrgTx(ctx, pool, orgB, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM secrets`).Scan(&countB)
	})
	require.NoError(t, err)
	require.Equal(t, 1, countB, "org B debe ver solo SU secret")
}

// Sabotaje (HU-25.5 escenario 1): query sin SET LOCAL → 0 rows.
// Esta es la prueba CLAVE: defense-in-depth contra bugs RBAC.
func TestSabotage_RLS_NoSetLocal_ZeroRows(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, _ := seedTwoOrgs(t, pool)

	// Insert con SET LOCAL OK
	err := txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'k1', 'v1', '\x00')`, orgA)
		return err
	})
	require.NoError(t, err)

	// Query directa SIN SET LOCAL → 0 rows (RLS deniega)
	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM secrets`).Scan(&count)
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
			`INSERT INTO secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'secret-b', 'B', '\x00') RETURNING id`, orgB).Scan(&secretIDB)
	})
	require.NoError(t, err)

	// App en context A intenta SELECT por id de B
	var found int
	err = txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM secrets WHERE id = $1`, secretIDB).Scan(&found)
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

	// App en context A intenta INSERT con organization_id=B (typo o bug)
	err := txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO secrets (organization_id, slug, name, encrypted_value)
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
				`INSERT INTO activity_log (organization_id, action, entity_type, summary)
				 VALUES ($1, 'test', 'x', 'evento')`, org)
			return err
		})
		require.NoError(t, err)
	}

	var countA int
	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM activity_log`).Scan(&countA)
	}))
	require.Equal(t, 1, countA)
}

// Escenario: api_keys también scoped.
func TestRLS_APIKeys_OrgIsolation(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, orgB := seedTwoOrgs(t, pool)

	// crear user en cada org para satisfacer FK
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
			`INSERT INTO api_keys (organization_id, user_id, key_hash, key_prefix, name)
			 VALUES ($1, $2, '\x00', 'domk_live_aaaaaaa', 'A1')`, orgA, uidA)
		return err
	}))
	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO api_keys (organization_id, user_id, key_hash, key_prefix, name)
			 VALUES ($1, $2, '\x00', 'domk_live_bbbbbbb', 'B1')`, orgB, uidB)
		return err
	}))

	var countA int
	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM api_keys`).Scan(&countA)
	}))
	require.Equal(t, 1, countA, "org A ve solo SU api_key")
}

// SET LOCAL muere al COMMIT (importante para pgbouncer transaction-pool).
func TestRLS_SetLocalScopedToTx(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()
	orgA, _ := seedTwoOrgs(t, pool)

	// Insert en tx con SET LOCAL
	require.NoError(t, txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO secrets (organization_id, slug, name, encrypted_value)
			 VALUES ($1, 'k', 'v', '\x00')`, orgA)
		return err
	}))

	// Después de commit, nueva query SIN SET LOCAL → 0 rows
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM secrets`).Scan(&count))
	require.Equal(t, 0, count, "post-tx sin SET LOCAL: RLS deniega")
}
