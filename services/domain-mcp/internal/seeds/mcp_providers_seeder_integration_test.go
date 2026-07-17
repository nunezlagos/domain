//go:build integration



package seeds_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
)

func TestSeeds_MCPProviders_BuiltInsCreated(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.MCPProvidersSeeder{})

	reports, err := reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)
	require.Equal(t, 6, reports["mcp_providers"].Created, "debe crear 6 built-ins")

	rows, err := pool.Query(ctx, `
		SELECT name, description, command
		FROM mcp_providers
		WHERE is_built_in = TRUE
		ORDER BY name
	`)
	require.NoError(t, err)
	defer rows.Close()

	got := map[string]string{}
	for rows.Next() {
		var name, desc, cmd string
		require.NoError(t, rows.Scan(&name, &desc, &cmd))
		got[name] = desc
	}

	expected := []string{"fetch", "filesystem", "git", "github", "memory", "time"}
	for _, name := range expected {
		require.Truef(t, got[name] != "", "built-in %s no creado", name)
	}
}

func TestSeeds_MCPProviders_Idempotent(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.MCPProvidersSeeder{})

	_, err := reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)

	_, err = reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM mcp_providers
	`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 6, count, "segunda corrida no debe duplicar")
}

func TestSeeds_MCPProviders_GitHubRequiresToken(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.MCPProvidersSeeder{})
	_, err := reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)

	var requiredEnv []string
	err = pool.QueryRow(ctx, `
		SELECT required_env FROM mcp_providers WHERE name = 'github'
	`).Scan(&requiredEnv)
	require.NoError(t, err)
	require.Contains(t, requiredEnv, "GITHUB_TOKEN", "github provider debe requerir GITHUB_TOKEN")
}
