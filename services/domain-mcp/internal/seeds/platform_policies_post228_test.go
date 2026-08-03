// issue-54.8: guards del CATÁLOGO para la reconciliación de las 4 platform policies que
// DOMAINSERV-228 midió divergentes. Estos son unitarios a propósito —verifican el fuente— y
// van acompañados de los de integración en platform_policies_post228_integration_test.go, que
// son los que miran la FILA. La lección de DOMAINSERV-228 es justamente que un guard del
// fuente puede estar verde con producción vieja: ninguno de estos dos reemplaza al otro.
package seeds_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
)

func policyDelCatalogo(t *testing.T, slug string) seeds.PolicyEntry {
	t.Helper()
	for _, p := range seeds.PlatformPolicyCatalog() {
		if p.Slug == slug {
			return p
		}
	}
	t.Fatalf("el slug %q no está en PlatformPolicyCatalog()", slug)
	return seeds.PolicyEntry{}
}

// El Corolario 2 vivía SOLO en la fila de producción: pisarla con el catálogo habría borrado
// ~35 líneas. Por eso el veredicto de este slug fue merge y no catálogo.
func TestPlatformPolicies_Catalogo_GuardsDebenEjecutarse_ContieneElCorolario2(t *testing.T) {
	body := policyDelCatalogo(t, "guards-deben-ejecutarse").BodyMD

	require.Contains(t, body, "## Corolario 2: una feature que nunca se ejecutó tampoco está cubierta",
		"el Corolario 2 solo existía en la fila de prod; sin él en el catálogo, el re-seed lo borra")
	require.Contains(t, body, "Cinco casos medidos el 2026-07-29",
		"el encabezado fechado es evidencia datada: se conserva aunque el estado de los casos cambie")
	require.Contains(t, body, "migrate.EmbeddingDim",
		"el texto nombraba llm.DefaultDim, símbolo que ya no existe en el repo")
	require.NotContains(t, body, "llm.DefaultDim",
		"una policy que cita un símbolo inexistente manda a buscar código que no está")
	require.NotContains(t, body, "# Un guard que no se ejecuta no es un guard",
		"el H1 es el campo Name; ninguna entrada del catálogo lo repite en el body")
}

// La dimensión que la policy afirma tiene que ser la real: si migrate.EmbeddingDim cambia y el
// texto queda en 1024, la policy vuelve a mandar a buscar un número que no existe. Es el mismo
// modo de falla que tenía con llm.DefaultDim.
func TestPlatformPolicies_Catalogo_GuardsDebenEjecutarse_CitaLaDimensionReal(t *testing.T) {
	body := policyDelCatalogo(t, "guards-deben-ejecutarse").BodyMD

	require.Contains(t, body, "vector(1024)",
		"el texto cita la dimensión del esquema; hoy migrate.EmbeddingDim = %d", migrate.EmbeddingDim)
	require.Equal(t, 1024, migrate.EmbeddingDim,
		"si la dimensión cambió, actualizar también el Corolario 2 de guards-deben-ejecutarse")
}

// La divergencia de sdd-auto-trigger eran 7 pares de backticks, y la causa era estructural: un
// raw-string de Go se delimita con backtick, así que no podía contenerlos. Por eso
// 000270_reconcile_stale_user_modified_policies:7-8 declaró esa edición legítima y la excluyó
// del reset. Este guard fija que el catálogo ya no tiene esa limitación.
func TestPlatformPolicies_Catalogo_SddAutoTrigger_TieneLosBackticksDeLaFila(t *testing.T) {
	body := policyDelCatalogo(t, "sdd-auto-trigger").BodyMD

	for _, esperado := range []string{
		"(`domain_orchestrate`)",
		"la fase `sdd-spec`;",
		"→ `domain_orchestrate` PRIMERO",
		"→ `domain_ticket_create`",
		"En la fase `sdd-spec`:",
		"El gate `hardspec` pausa",
		"RETOMARLO (`domain_flow_status`)",
	} {
		require.Contains(t, body, esperado,
			"falta un backtick que la fila de prod sí tiene: el slug volvería a diverger tras el reset")
	}

	require.Equal(t, 7, strings.Count(body, "`")/2,
		"son exactamente 7 pares; de más o de menos y el md5 contra la fila no coincide")
}

// El bump es lo que HABILITA la reconciliación. seeds.go:144 saltea el seeder cuando
// applied_version >= Version(), y producción está en 28: sin subirlo, la migración 000282
// limpia el flag y aun así el catálogo no llega a ningún ambiente. Es el paso que se olvida.
func TestPlatformPoliciesSeeder_Version_SuperaLaAplicadaEnProd(t *testing.T) {
	const aplicadaEnProd = 28

	require.Greater(t, (&seeds.PlatformPoliciesSeeder{}).Version(), aplicadaEnProd,
		"con Version() <= %d el seeder skippea y el reset del flag no reconcilia nada", aplicadaEnProd)
}
