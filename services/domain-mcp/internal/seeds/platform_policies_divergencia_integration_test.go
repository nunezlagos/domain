//go:build integration

// DOMAINSERV-228: medido en prod el 2026-08-03, 8 de 32 platform policies tienen
// is_user_modified = true, y el seeder no las pisa. Al menos una está VIEJA:
// `context-preservation` no tiene el fan-out del MUST-7 de DOMAINSERV-161, así que un fix
// mergeado y con test verde NO está en producción.
//
// Por qué ningún guard lo veía: los guards del paquete verifican el FUENTE —leen
// platform_policies_seeder.go como texto y buscan fragmentos—, y el fuente está bien. Lo que
// está mal es la fila. Un test unitario no puede ver esa diferencia; hace falta una base.
//
// Este guard compara la BD contra el CATÁLOGO y solo tolera la divergencia cuando la fila
// declara is_user_modified: ahí es una decisión del operador, no un fix perdido.
package seeds_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
)

// divergenciasNoDeclaradas devuelve los slugs cuya fila NO coincide con el catálogo y que
// tampoco declaran is_user_modified. Va extraída y no inline en el test para que se pueda
// plantar una divergencia y exigir que la encuentre: un guard que solo se ve pasar no está
// probado.
func divergenciasNoDeclaradas(ctx context.Context, t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	var out []string
	for _, p := range seeds.PlatformPolicyCatalog() {
		var body string
		var userModified bool
		err := pool.QueryRow(ctx,
			`SELECT body_md, is_user_modified FROM platform_policies WHERE slug = $1 AND is_active`,
			p.Slug).Scan(&body, &userModified)
		require.NoError(t, err, "la policy %q del catálogo no llegó a la BD", p.Slug)

		// una fila marcada la editó una persona y el seeder la respeta a propósito
		if !userModified && body != p.BodyMD {
			out = append(out, p.Slug)
		}
	}
	return out
}

func TestPlatformPolicies_NingunaFilaDivergeDelCatalogoSinDeclararlo(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)

	require.NotEmpty(t, seeds.PlatformPolicyCatalog(), "sin catálogo el guard queda verde por vacío")
	require.Empty(t, divergenciasNoDeclaradas(ctx, t, pool),
		"hay filas que divergen del catálogo sin declarar is_user_modified: son fixes que no llegaron a la BD")
}

// El sabotaje del guard, plantado: una fila vieja SIN el flag es exactamente el defecto que
// se midió en prod (context-preservation sin el fan-out de DOMAINSERV-161). El guard tiene
// que nombrarla.
func TestPlatformPolicies_FilaViejaSinElFlag_ElGuardLaEncuentra(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)
	require.Empty(t, divergenciasNoDeclaradas(ctx, t, pool), "arranca limpio")

	_, err := pool.Exec(ctx,
		`UPDATE platform_policies SET body_md = 'version vieja sin el fan-out'
		 WHERE slug = 'context-preservation' AND is_active`)
	require.NoError(t, err)

	require.Equal(t, []string{"context-preservation"}, divergenciasNoDeclaradas(ctx, t, pool),
		"el guard tiene que nombrar la fila divergente, y solo esa")
}

// La contra-cara: una fila marcada is_user_modified SÍ puede diverger, y el guard de arriba
// tiene que tolerarlo. Sin este test, el de arriba pasaría también con un `continue`
// incondicional y no probaría nada.
func TestPlatformPolicies_FilaMarcada_PuedeDivergerSinRomperElGuard(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)

	const slug = "context-preservation"
	_, err := pool.Exec(ctx,
		`UPDATE platform_policies SET body_md = 'editado a mano', is_user_modified = true
		 WHERE slug = $1 AND is_active`, slug)
	require.NoError(t, err)

	runPlatformPoliciesSeeder(ctx, t, pool)

	var body string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT body_md FROM platform_policies WHERE slug = $1 AND is_active`, slug).Scan(&body))
	require.Equal(t, "editado a mano", body,
		"el seeder respeta la edición del operador: eso es el contrato, no el bug")

	// y este es el punto del ticket: la fila queda divergente Y el re-seed la reporta como
	// preserved, que es la única señal de que hay una versión vieja gobernando
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	rep, err := (&seeds.PlatformPoliciesSeeder{}).Run(ctx, tx, seeds.EnvProd)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.GreaterOrEqual(t, rep.Preserved, 1,
		"una fila marcada tiene que contarse como preserved: es el canal que DOMAINSERV-225 arregló")
}
