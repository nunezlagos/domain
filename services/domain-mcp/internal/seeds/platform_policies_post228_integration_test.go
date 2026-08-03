//go:build integration

// issue-54.8: la migración 000282 devuelve al gobierno del seeder las 4 filas que
// DOMAINSERV-228 midió divergentes, y SOLO esas. El test es de integración y no unitario a
// propósito: el defecto que se está corrigiendo era invisible para los guards del fuente, y un
// reset mal acotado solo se ve contra una base con el estado plantado.
package seeds_test

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
)

// los 4 adjudicados por el análisis; el reset los alcanza
var slugsReconciliados = []string{
	"context-preservation",
	"guards-deben-ejecutarse",
	"reportar-consumo-de-memoria",
	"sdd-auto-trigger",
}

// marcado en prod PERO fuera del reset: su body está al día y el flag está de más, así que
// reconciliarlo no aporta y tocarlo sin adjudicación sería exceder el alcance. Es el
// discriminador del test: si el WHERE se ensancha, este slug lo delata.
const slugMarcadoNoAdjudicado = "delegar-lecturas-multiples"

func aplicarMigracion000282(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	mig, err := os.ReadFile("../migrate/migrations/000282_reconcile_platform_policies_post_228.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(mig))
	require.NoError(t, err)
}

func slugsMarcados(ctx context.Context, t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(ctx,
		`SELECT slug FROM platform_policies WHERE is_active AND is_user_modified`)
	require.NoError(t, err)
	defer rows.Close()

	var out []string
	for rows.Next() {
		var slug string
		require.NoError(t, rows.Scan(&slug))
		out = append(out, slug)
	}
	require.NoError(t, rows.Err())
	sort.Strings(out)
	return out
}

// planta el estado de producción: las 5 filas del catálogo que allá tienen is_user_modified.
func plantarFilasMarcadasDeProd(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`UPDATE platform_policies SET is_user_modified = true
		 WHERE is_active AND slug = ANY($1)`,
		append(append([]string{}, slugsReconciliados...), slugMarcadoNoAdjudicado))
	require.NoError(t, err)
}

func TestMigracion000282_ReseteaSoloLosCuatroSlugsAdjudicados(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)
	plantarFilasMarcadasDeProd(ctx, t, pool)
	require.Len(t, slugsMarcados(ctx, t, pool), 5, "arranca con el estado de prod plantado")

	aplicarMigracion000282(ctx, t, pool)

	require.Equal(t, []string{slugMarcadoNoAdjudicado}, slugsMarcados(ctx, t, pool),
		"la migración tiene que resetear los 4 adjudicados y NINGUNO más: un WHERE ancho "+
			"reabriría filas cuya edición sí era del operador")
}

// El sabotaje del guard, plantado: sin el WHERE la migración resetea todo. Sin este test, el
// de arriba pasaría igual con un UPDATE sin acotar mientras el catálogo tuviera 5 marcadas.
func TestMigracion000282_SinElWhereDeSlugs_ElGuardLoDelata(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)
	plantarFilasMarcadasDeProd(ctx, t, pool)

	// la versión saboteada: el reset ancho que la migración real NO hace
	_, err := pool.Exec(ctx,
		`UPDATE platform_policies SET is_user_modified = false WHERE is_active AND is_user_modified`)
	require.NoError(t, err)

	require.Empty(t, slugsMarcados(ctx, t, pool),
		"el sabotaje deja cero filas marcadas; la migración real deja una y por eso discrimina")
}

// El cierre del círculo de DOMAINSERV-228: tras el reset, el re-seed pisa las 4 filas con el
// catálogo. El bump de Version() es lo que lo permite — sin él seeds.go:144 skippea.
func TestPlatformPolicies_TrasLaMigracion000282_ElReseedReconciliaLasCuatro(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)
	plantarFilasMarcadasDeProd(ctx, t, pool)

	// el body viejo va en las 5 marcadas, no solo en las 4 adjudicadas: la quinta es la que
	// prueba que el seeder sigue respetando una fila que el reset no alcanzó
	const bodyViejo = "version vieja que quedo gobernando en prod"
	_, err := pool.Exec(ctx,
		`UPDATE platform_policies SET body_md = $1 WHERE is_active AND slug = ANY($2)`,
		bodyViejo, append(append([]string{}, slugsReconciliados...), slugMarcadoNoAdjudicado))
	require.NoError(t, err)

	aplicarMigracion000282(ctx, t, pool)
	runPlatformPoliciesSeeder(ctx, t, pool)

	for _, slug := range slugsReconciliados {
		require.Equal(t, policyDelCatalogo(t, slug).BodyMD, policyBody(ctx, t, pool, slug),
			"%s tiene que quedar igual al catálogo tras el re-seed", slug)
	}

	require.Equal(t, bodyViejo, policyBody(ctx, t, pool, slugMarcadoNoAdjudicado),
		"la fila marcada fuera del reset conserva su body: el seeder la sigue respetando")
}

// guards-deben-ejecutarse es el único de los 4 con contenido exclusivo en la fila. Este test
// fija el ORDEN del change: el Corolario 2 tiene que estar en el catálogo ANTES del reset, o
// el re-seed lo borra de producción.
func TestPlatformPolicies_TrasElReseed_ElCorolario2SobreviveEnLaFila(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)
	plantarFilasMarcadasDeProd(ctx, t, pool)
	aplicarMigracion000282(ctx, t, pool)
	runPlatformPoliciesSeeder(ctx, t, pool)

	require.Contains(t, policyBody(ctx, t, pool, "guards-deben-ejecutarse"),
		"## Corolario 2: una feature que nunca se ejecutó tampoco está cubierta",
		"el merge al catálogo tiene que preceder al reset: si no, este re-seed borra el Corolario")
}

// Contra-prueba de que la reconciliación cierra el hueco: tras el re-seed, el guard de
// divergencia de DOMAINSERV-228 no encuentra nada, y ya no porque las filas estén blindadas.
func TestPlatformPolicies_TrasLaReconciliacion_NoQuedaDivergenciaOculta(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)
	plantarFilasMarcadasDeProd(ctx, t, pool)
	aplicarMigracion000282(ctx, t, pool)
	runPlatformPoliciesSeeder(ctx, t, pool)

	require.NotEmpty(t, seeds.PlatformPolicyCatalog(), "sin catálogo el guard queda verde por vacío")
	require.Empty(t, divergenciasNoDeclaradas(ctx, t, pool), "no quedan fixes sin llegar a la BD")
	require.NotContains(t, slugsMarcados(ctx, t, pool), "context-preservation",
		"el caso con daño activo tiene que quedar bajo el gobierno del seeder, no tolerado por el flag")
}
