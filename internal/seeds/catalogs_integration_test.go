//go:build integration

package seeds_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
)

func setupSeedDB(t *testing.T) (pools *db.Pools, cleanup func()) {
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
	pools, err = db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	return pools, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestPlansSeeder_NoCommercialPricing(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.PlansSeeder{})
	_, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)

	// Invariante open-source: monthly_price_usd siempre 0.
	rows, err := pools.App.Query(ctx,
		`SELECT slug, monthly_price_usd FROM plans`)
	require.NoError(t, err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var slug string
		var price int
		require.NoError(t, rows.Scan(&slug, &price))
		require.Equalf(t, 0, price,
			"plan %s tiene monthly_price_usd=%d — Domain es open-source sin cobro (issue-21.4 archived)",
			slug, price)
		count++
	}
	require.GreaterOrEqual(t, count, 4)

	// Slugs nuevos (no comerciales) presentes.
	for _, want := range []string{"trial", "standard", "extended", "unlimited"} {
		var found bool
		err := pools.App.QueryRow(ctx,
			`SELECT TRUE FROM plans WHERE slug = $1`, want).Scan(&found)
		require.NoErrorf(t, err, "slug %s missing", want)
	}

	// Slugs comerciales viejos NO deben estar.
	for _, old := range []string{"free", "starter", "team", "enterprise"} {
		var count int
		require.NoError(t, pools.App.QueryRow(ctx,
			`SELECT COUNT(*) FROM plans WHERE slug = $1`, old).Scan(&count))
		require.Equalf(t, 0, count,
			"slug comercial viejo %s todavía existe — cleanup falló", old)
	}
}

// Sabotaje: si una org tiene asignado un plan legacy ("free" sembrado por
// migration 000032), el cleanup del seeder NO debe borrarlo — rompería el
// FK organizations.plan_id ON DELETE RESTRICT. Solo los huérfanos van.
func TestSabotage_PlansSeeder_PreservesAssignedLegacyPlan(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	// Migration 000032 ya creó "free"/"pro"/"enterprise". Tomo el id de
	// "free" y se lo asigno a una org.
	var legacyID uuid.UUID
	err := pools.App.QueryRow(ctx,
		`SELECT id FROM plans WHERE slug = 'free'`).Scan(&legacyID)
	require.NoError(t, err, "migration 000032 debió haber sembrado 'free'")
	_, err = pools.App.Exec(ctx,
		`INSERT INTO organizations (name, slug, plan_id) VALUES ('Acme', 'acme', $1)`,
		legacyID)
	require.NoError(t, err)

	// Corro el seeder.
	reg := seeds.NewRegistry()
	reg.Register(&seeds.PlansSeeder{})
	_, err = reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)

	// "free" SIGUE existiendo porque está referenciado por la org.
	var still int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM plans WHERE slug = 'free'`).Scan(&still))
	require.Equal(t, 1, still, "legacy plan asignado a org NO debe borrarse")

	// "pro" y "enterprise" SÍ deberían haberse borrado (no asignados).
	var pro int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM plans WHERE slug = 'pro'`).Scan(&pro))
	require.Equal(t, 0, pro, "pro legacy huérfano debió borrarse")
}

func TestModelRegistrySeeder_PopulatesCatalog(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.ModelRegistrySeeder{})
	results, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)
	require.Greater(t, results["model_registry"].Created, 10)

	var count int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM model_registry WHERE is_active = TRUE`).Scan(&count))
	require.GreaterOrEqual(t, count, 12, "al menos 12 modelos sembrados")

	// Verifica que Claude Opus 4.7 está
	var displayName string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT display_name FROM model_registry
		 WHERE provider='anthropic' AND model='claude-opus-4-7'`).Scan(&displayName))
	require.Equal(t, "Claude Opus 4.7", displayName)
}

func TestModelRegistrySeeder_Idempotent(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.ModelRegistrySeeder{})

	_, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)

	// Segunda corrida debe ser skip (mismo version)
	results, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)
	require.Equal(t, 1, results["model_registry"].Skipped,
		"segunda corrida debe skipear por version match")
}

func TestPlatformPoliciesSeeder_PopulatesBaseline(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.PlatformPoliciesSeeder{})
	results, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)
	require.Greater(t, results["platform_policies"].Created, 5)

	// Verifica que SDD TDD strict está presente
	var name string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT name FROM platform_policies WHERE slug='sdd-tdd-strict'`).Scan(&name))
	require.Contains(t, name, "TDD")
}

func TestSeedSkillsForOrg_BuiltinCatalog(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	// Crea org via insert directo (sin pasar por service)
	var orgID uuid.UUID
	err := pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID)
	require.NoError(t, err)

	rep, err := seeds.SeedSkillsForOrg(ctx, pools.App, orgID, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, rep.Created, 5, "al menos 5 skills built-in")

	var count int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM skills WHERE organization_id=$1 AND seed_managed=TRUE`,
		orgID).Scan(&count))
	require.GreaterOrEqual(t, count, 5)
}

func TestSeedAgentTemplatesForOrg_BuiltinCatalog(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgID uuid.UUID
	err := pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID)
	require.NoError(t, err)

	rep, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, rep.Created, 8, "10 agent templates built-in")

	// Verifica que el supervisor está
	var slug string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT slug FROM agent_templates WHERE organization_id=$1 AND slug='supervisor'`,
		orgID).Scan(&slug))
	require.Equal(t, "supervisor", slug)

	// Verifica que researcher tiene capabilities
	var caps []string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT capabilities FROM agent_templates WHERE organization_id=$1 AND slug='researcher'`,
		orgID).Scan(&caps))
	require.Contains(t, caps, "web-fetch")
}

// Sabotaje: SeedSkillsForOrg con is_user_modified=TRUE NO debe sobrescribir.
func TestSabotage_SkillsForOrg_PreservesUserModified(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgID uuid.UUID
	err := pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID)
	require.NoError(t, err)

	_, err = seeds.SeedSkillsForOrg(ctx, pools.App, orgID, 1)
	require.NoError(t, err)

	// Usuario customiza skill "summarize"
	_, err = pools.App.Exec(ctx,
		`UPDATE skills SET description = 'CUSTOM USER VERSION', is_user_modified = TRUE
		 WHERE organization_id = $1 AND slug = 'summarize'`, orgID)
	require.NoError(t, err)

	// Re-seed con bump de version
	rep, err := seeds.SeedSkillsForOrg(ctx, pools.App, orgID, 2)
	require.NoError(t, err)
	require.GreaterOrEqual(t, rep.Preserved, 1, "summarize debe estar preservado")

	var desc string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT description FROM skills WHERE organization_id=$1 AND slug='summarize'`,
		orgID).Scan(&desc))
	require.Equal(t, "CUSTOM USER VERSION", desc, "user modifications no se sobrescriben")
}
