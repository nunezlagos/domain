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

// REQ-42.2: TestPlansSeeder_* se removieron junto con el dominio billing/
// costos (tabla plans dropeada en 000148, PlansSeeder eliminado).

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

	// El protocolo de agente vive en BD (source-of-truth editable):
	// los agentes lo cargan con domain_policy_get('agent-protocol').
	var body string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT body_md FROM platform_policies WHERE slug='agent-protocol'`).Scan(&body))
	require.Contains(t, body, "domain tiene prioridad")
	require.Contains(t, body, "domain_mem_save")
}

func TestPlatformPoliciesSeeder_PreservesUserModified(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.PlatformPoliciesSeeder{})
	_, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)

	// Usuario edita una policy → is_user_modified=TRUE
	_, err = pools.App.Exec(ctx,
		`UPDATE platform_policies
		 SET body_md='CONTENIDO CUSTOM DEL USUARIO', is_user_modified=TRUE
		 WHERE slug='sdd-tdd-strict'`)
	require.NoError(t, err)

	// Re-corro el seeder directo (bypass del skip por version del registry)
	tx, err := pools.Auth.Begin(ctx)
	require.NoError(t, err)
	_, err = (&seeds.PlatformPoliciesSeeder{}).Run(ctx, tx, seeds.EnvDev)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	// Sabotaje: la edición del usuario NO debe pisarse
	var body string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT body_md FROM platform_policies WHERE slug='sdd-tdd-strict'`).Scan(&body))
	require.Equal(t, "CONTENIDO CUSTOM DEL USUARIO", body,
		"seeder no debe pisar policy con is_user_modified=TRUE")

	// Una policy NO modificada sí se re-sincroniza desde el catálogo
	var modified bool
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT is_user_modified FROM platform_policies WHERE slug='migration-safety'`).Scan(&modified))
	require.False(t, modified)
}

func TestProjectTemplatesSeeder_BuiltinsAndUserModified(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.ProjectTemplatesSeeder{})
	results, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)
	require.Equal(t, 4, results["project_templates"].Created)

	// Built-ins son públicos sin org y hay exactamente un default
	var publics, defaults int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE is_default)
		 FROM project_templates WHERE organization_id IS NULL AND is_public`).
		Scan(&publics, &defaults))
	require.Equal(t, 4, publics)
	require.Equal(t, 1, defaults)

	// Usuario edita go-backend → re-seed no lo pisa
	_, err = pools.App.Exec(ctx,
		`UPDATE project_templates
		 SET name='Mi Go Custom', is_user_modified=TRUE
		 WHERE organization_id IS NULL AND slug='go-backend'`)
	require.NoError(t, err)

	tx, err := pools.Auth.Begin(ctx)
	require.NoError(t, err)
	rep, err := (&seeds.ProjectTemplatesSeeder{}).Run(ctx, tx, seeds.EnvDev)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.Equal(t, 1, rep.Skipped, "go-backend editado debe skipearse")
	require.Equal(t, 3, rep.Updated)

	var name string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT name FROM project_templates
		 WHERE organization_id IS NULL AND slug='go-backend'`).Scan(&name))
	require.Equal(t, "Mi Go Custom", name, "edición del usuario preservada")
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
		`SELECT COUNT(*) FROM skills WHERE seed_managed=TRUE`,
	).Scan(&count))
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
	require.GreaterOrEqual(t, rep.Created, 8, "11 agent templates v3 (1 orchestrator + 10 workers)")

	// Verifica que sdd-orchestrator está (reemplazó a supervisor en v3)
	var slug string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT slug FROM agent_templates WHERE slug='sdd-orchestrator'`,
	).Scan(&slug))
	require.Equal(t, "sdd-orchestrator", slug)

	// Verifica que sdd-explore tiene capabilities (reemplazó a researcher en v3)
	var caps []string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT capabilities FROM agent_templates WHERE slug='sdd-explore'`,
	).Scan(&caps))
	require.Contains(t, caps, "code-search")
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
		 WHERE slug = 'summarize'`)
	require.NoError(t, err)

	// Re-seed con bump de version
	rep, err := seeds.SeedSkillsForOrg(ctx, pools.App, orgID, 2)
	require.NoError(t, err)
	require.GreaterOrEqual(t, rep.Preserved, 1, "summarize debe estar preservado")

	var desc string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT description FROM skills WHERE slug='summarize'`,
	).Scan(&desc))
	require.Equal(t, "CUSTOM USER VERSION", desc, "user modifications no se sobrescriben")
}
