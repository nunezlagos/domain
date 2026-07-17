//go:build integration




package migrate_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
)

// setupPG levanta Postgres con pgvector via testcontainers.
func setupPG(t *testing.T) (string, func()) {
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
	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn, func() { _ = pgC.Terminate(ctx) }
}

// Escenario 1: Migración up crea todas las tablas.
func TestMigrate_Up_CreatesAllTables(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()

	err := dmigrate.Up(dsn)
	require.NoError(t, err)

	v, dirty, err := dmigrate.Version(dsn)
	require.NoError(t, err)
	require.NotZero(t, v, "debe terminar en la última migración aplicada")
	require.False(t, dirty)


	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	require.NoError(t, err)
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		got[name] = true
	}

	expected := []string{
		"users", "auth_api_keys", "projects", "observations",
		"prompts", "knowledge_docs", "knowledge_chunks",
		"skills", "skill_versions", "agents", "flows", "flow_runs",
		"agent_runs", "crons", "webhooks", "webhook_deliveries", "audit_log",
		"auth_secrets", "project_templates", "project_merges",
		"schema_migrations",
	}




	for _, table := range expected {
		require.Truef(t, got[table], "tabla %s no creada", table)
	}
}

// Escenario 2: Extensiones pgvector + pgcrypto activas.
func TestMigrate_Up_ExtensionsLoaded(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	for _, ext := range []string{"vector", "pgcrypto"} {
		var exists bool
		err := conn.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname=$1)`, ext,
		).Scan(&exists)
		require.NoError(t, err)
		require.Truef(t, exists, "extension %s missing", ext)
	}
}

// Escenario 3: observations tiene embedding vector(1536) + content_tsv GIN.
func TestMigrate_Up_ObservationsHasVectorAndTSV(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)


	_, err = conn.Exec(ctx, `
		INSERT INTO projects (id, name, slug)
		VALUES (gen_random_uuid(), 'P', 'p');
	`)
	require.NoError(t, err)


	vec := make([]byte, 0, 1536*4+2)
	vec = append(vec, '[')
	for i := 0; i < 1536; i++ {
		if i > 0 {
			vec = append(vec, ',')
		}
		vec = append(vec, '0', '.', '1')
	}
	vec = append(vec, ']')

	var obsID string

	err = conn.QueryRow(ctx, `
		INSERT INTO knowledge_observations (project_id, content, embedding)
		SELECT p.id, 'hola mundo', $1::vector(1536)
		FROM projects p
		LIMIT 1
		RETURNING id::text;
	`, string(vec)).Scan(&obsID)
	require.NoError(t, err)
	require.NotEmpty(t, obsID)


	var matched bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM knowledge_observations
			WHERE content_tsv @@ plainto_tsquery('spanish', 'mundo')
		)
	`).Scan(&matched)
	require.NoError(t, err)
	require.True(t, matched, "content_tsv FTS should match")
}

// Escenario 4: Migración down -all elimina todo limpio.
func TestMigrate_DownAll_RemovesAllTables(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()

	require.NoError(t, dmigrate.Up(dsn))
	require.NoError(t, dmigrate.Down(dsn, -1))

	v, _, err := dmigrate.Version(dsn)
	require.NoError(t, err)
	require.Equal(t, uint(0), v, "after down -all, version is 0")

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx, `
		SELECT count(*) FROM pg_tables
		WHERE schemaname='public' AND tablename != 'schema_migrations'
	`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "all domain tables dropped")
}

// Escenario 5: Idempotencia — segundo Up no cambia nada.
func TestMigrate_Up_Idempotent(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))
	v1, _, _ := dmigrate.Version(dsn)
	require.NoError(t, dmigrate.Up(dsn))
	v2, _, _ := dmigrate.Version(dsn)
	require.Equal(t, v1, v2, "second up must not change version")
}

// Round-trip up → down → up.
func TestMigrate_RoundTrip(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))
	vBefore, _, _ := dmigrate.Version(dsn)
	require.NoError(t, dmigrate.Down(dsn, -1))
	require.NoError(t, dmigrate.Up(dsn))
	vAfter, _, _ := dmigrate.Version(dsn)


	require.NotZero(t, vAfter)
	require.Equal(t, vBefore, vAfter, "up→down→up debe volver a la última versión")
}

