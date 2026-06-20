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
		   WHERE role='orchestrator'`,
	).Scan(&count))
	require.Equal(t, 1, count, "seeder v3 inserta exactamente 1 sdd-orchestrator")

	// Sabotaje: intentar INSERT manual de un segundo orchestrator.
	// ISSUE-21.6 single-org: el UNIQUE INDEX ahora es global (no per-org),
	// así que el rechazo es independiente del organization_id del INSERT.
	_, err = pools.App.Exec(ctx, `
		INSERT INTO agent_templates
		  (organization_id, slug, name, system_prompt, model,
		   temperature, max_tokens, handoff_policy, role, seed_managed)
		VALUES ($1, 'rogue-orchestrator', 'Rogue', 'prompt', 'claude-opus-4-7',
		        0.2, 4096, 'forbid', 'orchestrator', false)`,
		orgID)
	require.Error(t, err, "UNIQUE INDEX debe rechazar segundo orchestrator")
	// El driver pgx envuelve el error con SQLSTATE 23505 (unique_violation).
	// Validamos que el mensaje refiera al index correcto.
	require.True(t,
		strings.Contains(err.Error(), "agent_templates_orchestrator_unique_idx") ||
			strings.Contains(err.Error(), "duplicate key") ||
			strings.Contains(err.Error(), "unique"),
		"error debe referir al UNIQUE INDEX, recibí: %v", err)

	// Sanity: sigue habiendo sólo 1 orchestrator (global)
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates
		   WHERE role='orchestrator'`,
	).Scan(&count))
	require.Equal(t, 1, count, "tras el INSERT rechazado, sigue habiendo 1 orchestrator")

	// phase-workers: sin límite (siguen siendo N por org, pero single-org
	// no distingue)
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_templates
		   WHERE role='phase-worker'`,
	).Scan(&count))
	require.GreaterOrEqual(t, count, 10, "phase-workers no están limitados por el UNIQUE parcial")
}

// Verifica que el UNIQUE permite orchestrators en orgs DISTINTAS — el
// parcial es por organization_id, no global.
func TestSabotage_UniqueOrchestratorAcrossOrgs(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	// ISSUE-21.6 single-org: este test verifica comportamiento per-org.
	// En single-org no aplica — el UNIQUE es global. Mantenemos el
	// test como documentacion histórica del comportamiento pre-single-org.
	_, _ = pools, ctx
	t.Skip("ISSUE-21.6: el UNIQUE partial per-org no aplica en single-org")
}
