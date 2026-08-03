// El paso 2 de context-preservation (domain_mem_context) corre en CADA turno, y hasta ahora
// no declaraba que las observaciones vienen truncadas. DOMAINSERV-161 corrigió el paso 3
// (mem_search) porque ahí el defecto se notó, pero proyectarContextoDeObservaciones
// (search_snippet.go:36-49) trunca EXACTAMENTE igual: mismo snippetBytes, mismo content_len.
//
// El guard lee el valor real del fuente en vez de hardcodearlo: una policy que cita un número
// que el código ya no usa manda a buscar algo que no existe — el modo de falla que tenía
// llm.DefaultDim y que costó una migración entera detectar.
package seeds_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// snippetBytes es privado y vive en otro paquete (mcpserver), así que el guard lo lee del
// fuente. Leer el texto es lo que ata la policy al código: importarlo no se puede.
func snippetBytesDelFuente(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("../mcp/server/search_snippet.go")
	require.NoError(t, err)

	m := regexp.MustCompile(`snippetBytes\s*=\s*(\d+)`).FindStringSubmatch(string(src))
	require.Len(t, m, 2, "no se pudo leer snippetBytes de search_snippet.go: el guard quedaría ciego")
	return m[1]
}

func TestPlatformPolicies_ContextPreservation_ElPaso2DeclaraQueMemContextTrunca(t *testing.T) {
	body := policyDelCatalogo(t, "context-preservation").BodyMD
	limite := snippetBytesDelFuente(t)

	paso2, _, encontrado := strings.Cut(body, "### Si el agente nota pérdida de hilo")
	require.True(t, encontrado, "el body tiene que conservar la sección post-compact")
	_, paso2, encontrado = strings.Cut(paso2, "2. `domain_mem_context")
	require.True(t, encontrado, "el paso 2 tiene que seguir siendo domain_mem_context")

	require.Contains(t, paso2, limite,
		"el paso 2 debe declarar el limite real de truncado (%s), igual que el paso 3", limite)
	require.Contains(t, paso2, "content_len",
		"sin content_len el agente no tiene como saber cuanto texto le falta")
	require.Contains(t, paso2, "domain_mem_get_observation",
		"declarar el truncado sin dar la salida deja al agente sabiendo que le falta y sin como pedirlo")
}

// El paso 2 corre SIEMPRE y el 3 solo post-compact: si el 3 quedara mejor documentado que el
// 2, el defecto sin declarar estaria en el camino caliente y no en el excepcional.
func TestPlatformPolicies_ContextPreservation_LosDosPasosCitanElMismoLimite(t *testing.T) {
	body := policyDelCatalogo(t, "context-preservation").BodyMD
	limite := snippetBytesDelFuente(t)

	require.Equal(t, 2, strings.Count(body, "domain_mem_get_observation"),
		"la salida al truncado tiene que estar en los dos pasos que truncan, no solo en el 3")
	require.GreaterOrEqual(t, strings.Count(body, limite), 2,
		"los pasos 2 y 3 truncan con el mismo snippetBytes: los dos tienen que citarlo")
}
