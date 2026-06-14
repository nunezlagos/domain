//go:build integration

// F1: mcp_providers table — catálogo de MCPs instalables por el cliente IA.
// Verifica schema esperado: id, name (unique), description, command,
// default_args, env_template jsonb, required_env text[], tags text[],
// is_built_in, is_public, organization_id, timestamps.

package migrate_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	dmigrate "nunezlagos/domain/internal/migrate"
)

func TestMigrate_Up_MCPProvidersTableExists(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var exists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM pg_tables
			WHERE schemaname='public' AND tablename='mcp_providers'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "tabla mcp_providers debe existir")
}

func TestMigrate_Up_MCPProvidersSchema(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_name = 'mcp_providers'
		ORDER BY ordinal_position
	`)
	require.NoError(t, err)
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var name, dtype string
		require.NoError(t, rows.Scan(&name, &dtype))
		cols[name] = true
		_ = dtype
	}

	required := []string{
		"id", "name", "description", "command", "default_args",
		"env_template", "required_env", "tags", "is_built_in",
		"is_public", "organization_id", "created_at", "updated_at",
	}
	for _, c := range required {
		require.Truef(t, cols[c], "column %s missing en mcp_providers", c)
	}
}

func TestMigrate_Up_MCPProvidersNameUnique(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `INSERT INTO organizations (name, slug) VALUES ('Org', 'org')`)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `
		INSERT INTO mcp_providers (name, description, command)
		SELECT 'filesystem', 'FS read/write', 'npx' FROM organizations LIMIT 1
	`)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `
		INSERT INTO mcp_providers (name, description, command)
		SELECT 'filesystem', 'dup', 'npx' FROM organizations LIMIT 1
	`)
	require.Error(t, err, "name duplicado en mcp_providers debe fallar")
}
