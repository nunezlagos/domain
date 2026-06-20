//go:build integration

// Tests integration para issue-08.10 foundation:
// - Migration 000075 (role + seed_managed)
// - Seeder v3 (10 sdd-* templates con cleanup defensivo)
// - UNIQUE INDEX orchestrator único por org
package e2e_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
)

func setupTemplatesV3(t *testing.T) (*pgxpool.Pool, uuid.UUID, func()) {
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

	orgID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, 'Test Org', 'test-org')
	`, orgID)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
	return pool, orgID, cleanup
}

// Escenario 1 + 2: Catálogo v3 con sdd-orchestrator + 10 phase-workers.
func TestAgentTemplates_SeederV3_InsertsSddPipeline(t *testing.T) {
	pool, orgID, cleanup := setupTemplatesV3(t)
	defer cleanup()
	ctx := context.Background()

	rep, err := seeds.SeedAgentTemplatesForOrg(ctx, pool, orgID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, rep.Created, 1, "primera pasada debe crear templates")

	// Verificar 1 orchestrator
	var orchCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates WHERE role='orchestrator'`,
	).Scan(&orchCount)
	require.NoError(t, err)
	require.Equal(t, 1, orchCount, "debe haber exactamente 1 orchestrator")

	// Verificar 10 phase-workers (sdd-explore...sdd-onboard)
	var workerCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates WHERE role='phase-worker' AND slug LIKE 'sdd-%'`,
	).Scan(&workerCount)
	require.NoError(t, err)
	require.Equal(t, 10, workerCount, "debe haber 10 phase-workers sdd-*")

	// Verificar el orchestrator slug
	var orchSlug string
	err = pool.QueryRow(ctx,
		`SELECT slug FROM agent_templates WHERE role='orchestrator'`,
	).Scan(&orchSlug)
	require.NoError(t, err)
	require.Equal(t, "sdd-orchestrator", orchSlug)
}

// UNIQUE INDEX parcial: imposible insertar 2 orchestrators por org.
func TestAgentTemplates_UniqueOrchestratorPerOrg(t *testing.T) {
	pool, orgID, cleanup := setupTemplatesV3(t)
	defer cleanup()
	ctx := context.Background()

	// Primero corro el seeder para insertar el sdd-orchestrator
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pool, orgID)
	require.NoError(t, err)

	// Intento insertar segundo orchestrator manual
	_, err = pool.Exec(ctx, `
		INSERT INTO agent_templates
		  (organization_id, slug, name, system_prompt, handoff_policy, role, seed_managed)
		VALUES ($1, 'fake-orchestrator', 'Fake', 'sys', 'forbid', 'orchestrator', false)
	`, orgID)
	require.Error(t, err, "UNIQUE INDEX parcial debe rechazar 2do orchestrator")
}

// Idempotencia: seeder ejecutado 2x no duplica + Updated > 0.
func TestAgentTemplates_SeederIdempotent(t *testing.T) {
	pool, orgID, cleanup := setupTemplatesV3(t)
	defer cleanup()
	ctx := context.Background()

	rep1, err := seeds.SeedAgentTemplatesForOrg(ctx, pool, orgID)
	require.NoError(t, err)
	require.Equal(t, 11, rep1.Created, "primera pasada inserta 11 (1 orch + 10 workers)")

	rep2, err := seeds.SeedAgentTemplatesForOrg(ctx, pool, orgID)
	require.NoError(t, err)
	require.Equal(t, 0, rep2.Created, "segunda pasada NO duplica")
	require.Equal(t, 11, rep2.Updated, "segunda pasada updates 11")

	// Confirmar total = 11
	var total int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates`).Scan(&total)
	require.NoError(t, err)
	require.Equal(t, 11, total)
}

// Cleanup defensivo: borra slugs legacy seed_managed=true sin runs activos.
func TestAgentTemplates_CleanupRemovesLegacy(t *testing.T) {
	pool, orgID, cleanup := setupTemplatesV3(t)
	defer cleanup()
	ctx := context.Background()

	// Simular slugs legacy seed_managed=true (lo que tenía el catálogo v2)
	for _, slug := range []string{"researcher", "coder", "tester"} {
		_, err := pool.Exec(ctx, `
			INSERT INTO agent_templates
			  (organization_id, slug, name, system_prompt, handoff_policy, role, seed_managed, is_user_modified)
			VALUES ($1, $2, $2, 'sys', 'allow', 'phase-worker', true, false)
		`, orgID, slug)
		require.NoError(t, err)
	}

	rep, err := seeds.SeedAgentTemplatesForOrg(ctx, pool, orgID)
	require.NoError(t, err)
	require.Equal(t, 3, rep.Deleted, "cleanup defensivo debe borrar los 3 legacy")

	// Verificar que los legacy no están
	var legacyCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates WHERE slug IN ('researcher','coder','tester')`,
	).Scan(&legacyCount)
	require.NoError(t, err)
	require.Equal(t, 0, legacyCount, "legacy borrados")
}

// Cleanup defensivo NO borra is_user_modified=true.
func TestAgentTemplates_CleanupPreservesUserModified(t *testing.T) {
	pool, orgID, cleanup := setupTemplatesV3(t)
	defer cleanup()
	ctx := context.Background()

	// Insertar 1 legacy con is_user_modified=true (custom del user)
	_, err := pool.Exec(ctx, `
		INSERT INTO agent_templates
		  (organization_id, slug, name, system_prompt, handoff_policy, role, seed_managed, is_user_modified)
		VALUES ($1, 'my-custom', 'My Custom', 'sys', 'allow', 'phase-worker', true, true)
	`, orgID)
	require.NoError(t, err)

	rep, err := seeds.SeedAgentTemplatesForOrg(ctx, pool, orgID)
	require.NoError(t, err)
	require.Equal(t, 0, rep.Deleted, "is_user_modified=true NO debe borrarse")

	// Verificar que my-custom sigue ahí
	var customCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates WHERE slug='my-custom'`,
	).Scan(&customCount)
	require.NoError(t, err)
	require.Equal(t, 1, customCount, "customización del user preservada")
}
