//go:build integration

// DOMAINSERV-225: el contador del seeder de skills reportaba todo como Created. Decidía con
// tag.RowsAffected(), y un INSERT ... ON CONFLICT ... DO UPDATE devuelve 1 tanto para el
// insert como para el update, así que la rama Updated era código muerto.
//
// No es cosmético: created/updated/preserved es el ÚNICO canal por el que se detecta la
// trampa de is_user_modified —el seeder no pisa la fila, la cuenta como preserved, y el
// deploy se ve exitoso mientras la versión vieja sigue gobernando en la BD—. Con todo
// cayendo en Created, un preserved real queda invisible. Medido en prod el 2026-08-03: 8 de
// 32 platform policies están en ese estado (DOMAINSERV-228), así que el canal hace falta.
//
// El test es de INTEGRACIÓN y no unitario a propósito: la diferencia entre insert y update
// la sabe Postgres (xmax), no el código Go.
package seeds_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
)

func correrSeederDeSkills(ctx context.Context, t *testing.T, pool *pgxpool.Pool) seeds.Report {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	rep, err := (&seeds.SkillsCatalogSeeder{}).Run(ctx, tx, seeds.EnvProd)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	return rep
}

func TestSkillsCatalog_SegundaCorrida_ReportaUpdatedYNoCreated(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	primera := correrSeederDeSkills(ctx, t, pool)
	require.Greater(t, primera.Created, 0, "la primera corrida inserta: sin esto el resto no prueba nada")
	require.Equal(t, 0, primera.Updated, "en la primera corrida no hay nada que actualizar")

	segunda := correrSeederDeSkills(ctx, t, pool)

	// el bug: RowsAffected() da 1 para el UPDATE del upsert, así que todo caía en Created
	require.Equal(t, 0, segunda.Created,
		"la segunda corrida no inserta nada: reportar Created es el bug de DOMAINSERV-225")
	require.Equal(t, primera.Created, segunda.Updated,
		"la segunda corrida actualiza las mismas filas que la primera insertó")
}

func TestSkillsCatalog_FilaEditadaPorElUsuario_SeReportaPreserved(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	primera := correrSeederDeSkills(ctx, t, pool)

	_, err := pool.Exec(ctx,
		`UPDATE skills SET is_user_modified = true WHERE slug = 'commit-message' AND project_id IS NULL`)
	require.NoError(t, err)

	segunda := correrSeederDeSkills(ctx, t, pool)

	require.Equal(t, 1, segunda.Preserved,
		"una fila is_user_modified se cuenta como preserved: es la única señal de que el seeder NO la pisó")
	require.Equal(t, primera.Created-1, segunda.Updated,
		"el resto se actualiza normalmente")
	require.Equal(t, 0, segunda.Created, "nada se inserta en la segunda corrida")
}
