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

func TestPlatformPoliciesSeeder_PopulatesBaseline(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	reg := seeds.NewRegistry()
	reg.Register(&seeds.PlatformPoliciesSeeder{})
	results, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)
	require.Greater(t, results["platform_policies"].Created, 5)

	var name string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT name FROM platform_policies WHERE slug='sdd-tdd-strict'`).Scan(&name))
	require.Contains(t, name, "TDD")

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

	_, err = pools.App.Exec(ctx,
		`UPDATE platform_policies
		 SET body_md='CONTENIDO CUSTOM DEL USUARIO', is_user_modified=TRUE
		 WHERE slug='sdd-tdd-strict'`)
	require.NoError(t, err)

	tx, err := pools.Auth.Begin(ctx)
	require.NoError(t, err)
	_, err = (&seeds.PlatformPoliciesSeeder{}).Run(ctx, tx, seeds.EnvDev)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	var body string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT body_md FROM platform_policies WHERE slug='sdd-tdd-strict'`).Scan(&body))
	require.Equal(t, "CONTENIDO CUSTOM DEL USUARIO", body,
		"seeder no debe pisar policy con is_user_modified=TRUE")

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
	// Tras el refactor a meta-template, solo se siembra el built-in "default".
	require.Equal(t, 1, results["project_templates"].Created)

	var publics, defaults int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE is_default)
		 FROM project_templates WHERE organization_id IS NULL AND is_public`).
		Scan(&publics, &defaults))
	require.Equal(t, 1, publics)
	require.Equal(t, 1, defaults)

	_, err = pools.App.Exec(ctx,
		`UPDATE project_templates
		 SET name='Mi Default Custom', is_user_modified=TRUE
		 WHERE organization_id IS NULL AND slug='default'`)
	require.NoError(t, err)

	tx, err := pools.Auth.Begin(ctx)
	require.NoError(t, err)
	rep, err := (&seeds.ProjectTemplatesSeeder{}).Run(ctx, tx, seeds.EnvDev)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.Equal(t, 1, rep.Skipped, "default editado debe skipearse")
	require.Equal(t, 0, rep.Updated)

	var name string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT name FROM project_templates
		 WHERE organization_id IS NULL AND slug='default'`).Scan(&name))
	require.Equal(t, "Mi Default Custom", name, "edición del usuario preservada")
}

func TestSeedSkillsForOrg_BuiltinCatalog(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

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

	var slug string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT slug FROM agent_templates WHERE slug='sdd-orchestrator'`,
	).Scan(&slug))
	require.Equal(t, "sdd-orchestrator", slug)

	var caps []string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT capabilities FROM agent_templates WHERE slug='sdd-explore'`,
	).Scan(&caps))
	require.Contains(t, caps, "code-search")
}

// Guard DOMAINSERV-15/18: la sección de seguridad shift-left se HORNEA en el
// system_prompt del agent_template (no es skill runtime). Se pone ROJO si alguien
// borra la sección en cualquiera de las 3 fases SDD. Para sdd-4r los marcadores YA
// existen (R1 Risk + contrato_de_evidencia); sdd-spec/sdd-design son rojo-primero
// hasta hornear el bloque seguridad_shift_left y bumpear agentTemplatesSeedVersion.
func TestSeedAgentTemplatesForOrg_SystemPrompt_ShiftLeftSecuritySection_Present(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, uuid.New())
	require.NoError(t, err)

	cases := []struct {
		slug    string
		markers []string
	}{
		{slug: "sdd-4r", markers: []string{"seguridad", "causal_disposition", "proof_refs"}},
		{slug: "sdd-spec", markers: []string{"seguridad_shift_left", "causal_disposition", "pre-existing"}},
		{slug: "sdd-design", markers: []string{"seguridad_shift_left", "causal_disposition", "pre-existing"}},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			var prompt string
			require.NoError(t, pools.App.QueryRow(ctx,
				`SELECT system_prompt FROM agent_templates WHERE slug=$1`, tc.slug,
			).Scan(&prompt))
			for _, m := range tc.markers {
				require.Contains(t, prompt, m,
					"%s: falta el marcador de seguridad shift-left %q — ¿se borró la sección? (DOMAINSERV-15)",
					tc.slug, m)
			}
		})
	}
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

	_, err = pools.App.Exec(ctx,
		`UPDATE skills SET description = 'CUSTOM USER VERSION', is_user_modified = TRUE
		 WHERE slug = 'summarize'`)
	require.NoError(t, err)

	rep, err := seeds.SeedSkillsForOrg(ctx, pools.App, orgID, 2)
	require.NoError(t, err)
	require.GreaterOrEqual(t, rep.Preserved, 1, "summarize debe estar preservado")

	var desc string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT description FROM skills WHERE slug='summarize'`,
	).Scan(&desc))
	require.Equal(t, "CUSTOM USER VERSION", desc, "user modifications no se sobrescriben")
}
