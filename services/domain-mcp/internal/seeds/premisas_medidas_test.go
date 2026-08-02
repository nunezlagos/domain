package seeds

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-220: la regla del schema efectivo tiene que llegar al agente que escribe
// SQL, y ese agente puede ser un subagente que nunca vio agentprotocol.Full (Full se
// stubbea en el rulesBlock por tamaño). Por eso va como platform policy chica: bajo el
// maxInlinePolicyBody=4000 del orchestrator viaja inline en cada fase.
//
// Lee el fuente porque el catálogo vive inline en Run(), igual que reportar_consumo_memoria_test.
func TestPlatformPolicies_PremisasMedidas_EstaEnElCatalogo(t *testing.T) {
	b, err := os.ReadFile("platform_policies_seeder.go")
	require.NoError(t, err)
	src := string(b)

	i := strings.Index(src, `Slug:       "premisas-medidas-no-inferidas"`)
	require.GreaterOrEqual(t, i, 0,
		"la policy tiene que estar en el catálogo del seeder para llegar a un install limpio")

	cuerpo := src[i:]
	if fin := strings.Index(cuerpo[1:], "\t\t\tSlug:"); fin > 0 {
		cuerpo = cuerpo[:fin]
	}

	require.LessOrEqual(t, len(cuerpo), 4000,
		"si el body supera maxInlinePolicyBody deja de viajar inline y la regla no llega "+
			"al subagente que escribe SQL, que es el único motivo por el que es policy aparte")
	require.Contains(t, cuerpo, "HIPÓTESIS",
		"la regla tiene que separar lo medido de lo inferido: las 3 premisas falsas del "+
			"2026-07-31 eran inferencias con el tono de una medición")
	require.Contains(t, cuerpo, "000142",
		"tiene que nombrar la migración del DO anónimo, que es la que un grep no encuentra")
	require.NotContains(t, cuerpo, "000141, la 000142 y la 000143",
		"la 000141 y la 000143 SÍ nombran sus tablas: apuntar a las tres enseña algo falso")
}

// Sin bump el seeder skippea y ninguna de las dos doctrinas llega a un ambiente que ya
// corrió una versión anterior — y el skip es silencioso, indistinguible del éxito.
func TestPlatformPoliciesSeedVersion_CubreLaTandaDe220Y158(t *testing.T) {
	require.GreaterOrEqual(t, (&PlatformPoliciesSeeder{}).Version(), 28,
		"220 (policy nueva) y 158 (nueva sección de agent-protocol) comparten UN solo bump")
}
