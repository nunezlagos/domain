//go:build integration

// F3: inventory service — tests integration con testcontainers + seeders.

package inventory_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/inventory"
)

func setup(t *testing.T) (*pgxpool.Pool, string, func()) {
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

	reg := seeds.NewRegistry()
	reg.Register(&seeds.ProjectTemplatesSeeder{})
	reg.Register(&seeds.MCPProvidersSeeder{})
	reg.Register(&seeds.PlatformPoliciesSeeder{})
	_, err = reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)

	orgID, err := createOrg(ctx, pool)
	require.NoError(t, err)

	return pool, orgID, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func createOrg(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Test Org', 'test-org') RETURNING id::text
	`).Scan(&id)
	return id, err
}

func TestInventory_Load_BuiltinsAlwaysPresent(t *testing.T) {
	pool, _, cleanup := setup(t)
	defer cleanup()

	svc := inventory.New(pool)
	inv, err := svc.Load(context.Background(), inventory.LoadInput{OrgID: nil})
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(inv.MCPProviders), 6, "debe traer los 6 built-ins de mcp_providers")
	require.GreaterOrEqual(t, len(inv.Templates), 4, "debe traer los 4 built-ins de project_templates")
	require.GreaterOrEqual(t, len(inv.Policies), 1, "debe traer al menos 1 policy built-in")
}

func TestInventory_Load_MCPProvidersDetails(t *testing.T) {
	pool, _, cleanup := setup(t)
	defer cleanup()

	svc := inventory.New(pool)
	inv, err := svc.Load(context.Background(), inventory.LoadInput{})
	require.NoError(t, err)

	var found *inventory.MCPProviderSummary
	for i := range inv.MCPProviders {
		if inv.MCPProviders[i].Name == "github" {
			found = &inv.MCPProviders[i]
		}
	}
	require.NotNil(t, found, "github provider debe estar en inventory")
	require.Equal(t, "npx", found.Command)
	require.Contains(t, found.RequiredEnv, "GITHUB_TOKEN")
}

func TestInventory_Load_EmptyForNoOrg(t *testing.T) {
	pool, _, cleanup := setup(t)
	defer cleanup()

	svc := inventory.New(pool)
	inv, err := svc.Load(context.Background(), inventory.LoadInput{OrgID: nil})
	require.NoError(t, err)

	require.Empty(t, inv.Agents, "sin orgID no debe traer agents")
	require.Empty(t, inv.Skills, "sin orgID no debe traer skills")
	require.Empty(t, inv.Flows, "sin orgID no debe traer flows")
	require.Empty(t, inv.MCPServers, "sin orgID no debe traer mcp_servers")
}

func TestInventory_Load_OrgScopedEmpty(t *testing.T) {
	pool, orgID, cleanup := setup(t)
	defer cleanup()

	svc := inventory.New(pool)
	inv, err := svc.Load(context.Background(), inventory.LoadInput{OrgID: &orgID})
	require.NoError(t, err)

	require.Empty(t, inv.Agents, "org nueva sin agents per-org")
	require.Empty(t, inv.Skills, "org nueva sin skills per-org")
	require.Empty(t, inv.Flows, "org nueva sin flows per-org")
	require.NotEmpty(t, inv.MCPProviders, "built-ins sí están aunque no haya org")
}

func TestInventory_Load_TemplatesBuiltins(t *testing.T) {
	pool, _, cleanup := setup(t)
	defer cleanup()

	svc := inventory.New(pool)
	inv, err := svc.Load(context.Background(), inventory.LoadInput{})
	require.NoError(t, err)

	gotSlugs := map[string]bool{}
	for _, t := range inv.Templates {
		gotSlugs[t.Slug] = true
	}
	for _, expected := range []string{"default", "go-backend", "python-data", "frontend-web"} {
		require.Truef(t, gotSlugs[expected], "template built-in %s no presente", expected)
	}
}
