//go:build integration

package seeds_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
)

func TestSeedFlowsForOrg_CreatesSDDPipeline(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgID uuid.UUID
	err := pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID)
	require.NoError(t, err)

	rep, err := seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Created, "primera corrida crea sdd-pipeline-v1")
	require.Equal(t, 0, rep.Updated)
	require.Equal(t, 0, rep.Deleted)

	// Verifica row + flags + spec
	var (
		slug          string
		seedManaged   bool
		seedVersion   int
		userModified  bool
		specRaw       []byte
	)
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT slug, seed_managed, seed_version, is_user_modified, spec
		   FROM flows WHERE organization_id=$1 AND slug=$2`,
		orgID, seeds.SDDPipelineFlowSlug,
	).Scan(&slug, &seedManaged, &seedVersion, &userModified, &specRaw))
	require.Equal(t, seeds.SDDPipelineFlowSlug, slug)
	require.True(t, seedManaged, "flow seedeado debe estar marcado seed_managed=true")
	require.False(t, userModified, "fresh seed → is_user_modified=false")
	require.Equal(t, 1, seedVersion)

	// Spec debe tener los 10 steps en orden
	var spec seeds.FlowSpecJSON
	require.NoError(t, json.Unmarshal(specRaw, &spec))
	require.Len(t, spec.Steps, 10)
	for i, ph := range seeds.SDDPipelinePhaseSlugs {
		require.Equal(t, ph, spec.Steps[i].ID)
	}
}

func TestSeedFlowsForOrg_Idempotent(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID))

	rep1, err := seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	require.Equal(t, 1, rep1.Created)

	rep2, err := seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	require.Equal(t, 0, rep2.Created, "segunda corrida no inserta")
	require.Equal(t, 1, rep2.Updated, "segunda corrida hace UPDATE de la row existente")

	// Sigue habiendo exactamente 1 row
	var count int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM flows WHERE organization_id=$1`, orgID).Scan(&count))
	require.Equal(t, 1, count)
}

// Sabotage sab-equivalente: una vez que un usuario customiza el flow
// (is_user_modified=true), el seeder NO debe pisarlo. Reportado como
// Preserved en vez de Updated.
func TestSeedFlowsForOrg_PreservesUserModifications(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID))

	_, err := seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	// Usuario edita el name + marca is_user_modified=true
	_, err = pools.App.Exec(ctx,
		`UPDATE flows
		   SET name = 'CUSTOM PIPELINE',
		       is_user_modified = true
		 WHERE organization_id=$1 AND slug=$2`,
		orgID, seeds.SDDPipelineFlowSlug)
	require.NoError(t, err)

	rep, err := seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	require.Equal(t, 0, rep.Created)
	require.Equal(t, 1, rep.Preserved, "user-modified flow no se sobrescribe → Preserved")
	require.Equal(t, 0, rep.Updated)

	var name string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT name FROM flows WHERE organization_id=$1 AND slug=$2`,
		orgID, seeds.SDDPipelineFlowSlug).Scan(&name))
	require.Equal(t, "CUSTOM PIPELINE", name, "customización debe sobrevivir reseed")
}

// Cleanup defensivo: una row seed_managed con slug ya no presente en el
// catálogo (ej. seeder anterior con slug 'sdd-pipeline-legacy') debe
// borrarse si NO tiene flow_runs activos.
func TestSeedFlowsForOrg_CleansLegacySlugsWithoutActiveRuns(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID))

	// Inserto manualmente un flow legacy marcado seed_managed con
	// un slug que NO está en el catálogo actual
	specJSON := `{"version":1,"steps":[{"id":"x","type":"agent_run","config":{}}]}`
	_, err := pools.App.Exec(ctx,
		`INSERT INTO flows
		   (organization_id, slug, name, spec, is_active, seed_managed, seed_version)
		 VALUES ($1, 'legacy-removed', 'Legacy', $2::jsonb, true, true, 1)`,
		orgID, specJSON)
	require.NoError(t, err)

	rep, err := seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Created)
	require.Equal(t, 1, rep.Deleted, "legacy-removed se limpia")

	// Verifica que legacy se borró y sdd-pipeline está
	var slugs []string
	rows, err := pools.App.Query(ctx,
		`SELECT slug FROM flows WHERE organization_id=$1 ORDER BY slug`, orgID)
	require.NoError(t, err)
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		slugs = append(slugs, s)
	}
	rows.Close()
	require.Equal(t, []string{seeds.SDDPipelineFlowSlug}, slugs)
}

// Cleanup defensivo NO debe borrar flows legacy con flow_runs activos:
// rompería FK + perdería traza histórica. El seeder debe loggear y dejar
// la row en su lugar para que un humano decida.
func TestSeedFlowsForOrg_KeepsLegacyWithActiveRuns(t *testing.T) {
	pools, cleanup := setupSeedDB(t)
	defer cleanup()
	ctx := context.Background()

	var orgID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID))

	var legacyFlowID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO flows
		   (organization_id, slug, name, spec, is_active, seed_managed, seed_version)
		 VALUES ($1, 'legacy-running', 'Legacy Running',
		         '{"version":1,"steps":[{"id":"x","type":"agent_run","config":{}}]}'::jsonb,
		         true, true, 1)
		 RETURNING id`,
		orgID,
	).Scan(&legacyFlowID))

	// Crear un flow_run activo apuntando a este flow legacy
	_, err := pools.App.Exec(ctx,
		`INSERT INTO flow_runs (organization_id, flow_id, status)
		 VALUES ($1, $2, 'running')`,
		orgID, legacyFlowID)
	require.NoError(t, err)

	rep, err := seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	require.Equal(t, 0, rep.Deleted, "no se borra legacy con run activo")

	// Sigue existiendo
	var count int
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM flows WHERE organization_id=$1 AND slug='legacy-running'`,
		orgID).Scan(&count))
	require.Equal(t, 1, count)
}
