package seeds

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-145: la policy se creó primero por MCP, así que vive en la BD de
// producción pero no en el catálogo del seeder. Un install limpio la dejaría afuera y
// el protocolo de reporte no existiría en ese ambiente — mismo agujero que la versión
// 22 del seeder tuvo que tapar para context-preservation.
//
// Lee el fuente porque el catálogo vive inline en Run(), igual que voseo_guard_test.
func TestPlatformPolicies_ReportarConsumoDeMemoria_EstaEnElCatalogo(t *testing.T) {
	b, err := os.ReadFile("platform_policies_seeder.go")
	require.NoError(t, err)
	src := string(b)

	i := strings.Index(src, `Slug:       "reportar-consumo-de-memoria"`)
	require.GreaterOrEqual(t, i, 0,
		"la policy tiene que estar en el catálogo del seeder, no solo en la BD")

	cuerpo := src[i:]
	if fin := strings.Index(cuerpo[1:], "\t\t\tSlug:"); fin > 0 {
		cuerpo = cuerpo[:fin]
	}

	require.Contains(t, cuerpo, "candidate_ids",
		"la regla tiene que nombrar el denominador: sin él no hay tasa que medir")
	require.Contains(t, cuerpo, "domain_mem_used",
		"la regla tiene que nombrar la tool que se reporta")
	require.Contains(t, cuerpo, "no hay dato",
		"la regla tiene que fijar que no reportar es SIN DATO, nunca 'no sirvieron'")
}

// Sin bump, seeds.go skippea el seeder y la policy nueva no llega a ningún ambiente
// que ya haya corrido una versión anterior.
func TestPlatformPoliciesSeedVersion_CubreReportarConsumoDeMemoria(t *testing.T) {
	b, err := os.ReadFile("platform_policies_seeder.go")
	require.NoError(t, err)

	require.Contains(t, string(b), "return 27",
		"sembrar la policy nueva exige bump de PlatformPoliciesSeeder a 27")
}
