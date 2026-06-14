//go:build integration

package seeds_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
)

// sab-002 (issue-08.10): el UNIQUE INDEX parcial creado por la migración
// 000075 (`agent_templates_orchestrator_unique_idx WHERE role='orchestrator'`)
// debe impedir que coexistan dos agent_templates con role='orchestrator'
// en la misma org. Esta es la garantía estructural de RFC 0006 ADR-1:
// "el orquestador SDD es único entry point por org".
//
// El test ejecuta el seeder v3 (crea 1 sdd-orchestrator) y luego intenta
// INSERTar manualmente un segundo orchestrator. La query debe fallar con
// el error de unique violation de Postgres.
func TestSabotage_UniqueOrchestratorPerOrg(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID))

	// Seedea el catálogo v3 — crea exactamente 1 sdd-orchestrator
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	var count int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates
		   WHERE organization_id=$1 AND role='orchestrator'`,
		orgID).Scan(&count))
	require.Equal(t, 1, count, "seeder v3 inserta exactamente 1 sdd-orchestrator")

	// Sabotaje: intentar INSERT manual de un segundo orchestrator
	_, err = pools.App.Exec(ctx, `
		INSERT INTO agent_templates
		  (organization_id, slug, name, system_prompt, model,
		   temperature, max_tokens, handoff_policy, role, seed_managed)
		VALUES ($1, 'rogue-orchestrator', 'Rogue', 'prompt', 'claude-opus-4-7',
		        0.2, 4096, 'forbid', 'orchestrator', false)`,
		orgID)
	require.Error(t, err, "UNIQUE INDEX parcial debe rechazar segundo orchestrator")
	// El driver pgx envuelve el error con SQLSTATE 23505 (unique_violation).
	// Validamos que el mensaje refiera al index correcto.
	require.True(t,
		strings.Contains(err.Error(), "agent_templates_orchestrator_unique_idx") ||
			strings.Contains(err.Error(), "duplicate key") ||
			strings.Contains(err.Error(), "unique"),
		"error debe referir al UNIQUE INDEX parcial, recibí: %v", err)

	// Sanity: sigue habiendo sólo 1 orchestrator
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates
		   WHERE organization_id=$1 AND role='orchestrator'`,
		orgID).Scan(&count))
	require.Equal(t, 1, count, "tras el INSERT rechazado, sigue habiendo 1 orchestrator")

	// El INDEX es parcial — phase-workers SÍ pueden ser N por org
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates
		   WHERE organization_id=$1 AND role='phase-worker'`,
		orgID).Scan(&count))
	require.GreaterOrEqual(t, count, 10, "phase-workers no están limitados por el UNIQUE parcial")
}

// Verifica que el UNIQUE permite orchestrators en orgs DISTINTAS — el
// parcial es por organization_id, no global.
func TestSabotage_UniqueOrchestratorAcrossOrgs(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgA, orgB uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgA))
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Beta', 'beta') RETURNING id`,
	).Scan(&orgB))

	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgA)
	require.NoError(t, err)
	_, err = seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgB)
	require.NoError(t, err, "seedear orchestrator en org distinta NO debe fallar")

	var count int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates WHERE role='orchestrator'`,
	).Scan(&count))
	require.Equal(t, 2, count, "2 orchestrators en orgs distintas son válidos")
}
