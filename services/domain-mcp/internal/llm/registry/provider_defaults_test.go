package registry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/llm/google"
	"nunezlagos/domain/internal/llm/ollama"
	"nunezlagos/domain/internal/llm/openai"
	"nunezlagos/domain/internal/llm/registry"
)

// DOMAINSERV-183: cada provider tiene un defaultModel que se usa cuando
// CompletionOptions.Model viene vacío. Si ese ID no está en el registry de pricing, la
// corrida NO falla: el runner descarta ErrModelNotFound y queda contabilizada con
// COSTO 0. Es el mismo modo de falla que DOMAINSERV-162, que corrigió el registry pero
// no miró los defaults de los providers.
//
// El hueco es estructural: el guard que dejó DOMAINSERV-159
// (TestAgentTemplateCatalog_TodoModeloDeclarado_EstaCotizadoEnElRegistry) recorre solo
// AgentTemplateCatalog(), y los defaults de los providers viven fuera del catálogo. Sin
// este test nadie los mira.
//
// Se leen vía New().Model —el campo es público y New() lo inicializa con la constante
// privada— así que el guard no obliga a exportar nada.

type providerDefault struct {
	provider string
	modelID  string
}

func defaultsDeLosProviders() []providerDefault {
	return []providerDefault{
		{"anthropic", anthropic.New("").Model},
		{"openai", openai.New("").Model},
		{"ollama", ollama.New().Model},
		{"google", google.New("").Model},
	}
}

// defaultsConDeudaConocida son los que HOY apuntan a un ID no cotizado y cuyo arreglo
// NO es un cambio de precio sino de MODELO, así que no se decide en un guard:
//   - google: el default es gemini-2.0-flash y el registry solo cotiza 1.5-pro y
//     1.5-flash. Apuntarlo a 1.5 sería DEGRADAR el modelo; cotizar 2.0 requiere su
//     precio real, que no se inventa.
//   - ollama: el default es llama3.1 y el registry cotiza llama3.3:70b y llama3.2:3b.
//     Corre local, así que el costo 0 no es un error de contabilidad como en una API
//     remota — pero el ID igual debería existir en el registry para que el reporte no
//     mienta por omisión.
//
// Congelados acá y no arreglados en silencio: son decisiones de producto, no de lint.
// Ver DOMAINSERV-183.
var defaultsConDeudaConocida = map[string]bool{
	"google": true,
	"ollama": true,
}

// Un default no cotizado hace que toda corrida que caiga en él se contabilice con costo
// 0, sin error ni log. El reporte de gasto miente hacia abajo y nada lo señala.
func TestProviderDefaults_TodoDefaultEstaCotizadoEnElRegistry(t *testing.T) {
	reg := registry.New()

	var evaluados int
	for _, d := range defaultsDeLosProviders() {
		if defaultsConDeudaConocida[d.provider] {
			continue
		}
		evaluados++
		_, err := reg.Get(context.Background(), d.provider, d.modelID)
		require.NoErrorf(t, err,
			"el defaultModel de %s es %q y NO está en el registry: toda corrida que caiga en ese default se contabiliza con costo 0",
			d.provider, d.modelID)
	}
	require.NotZero(t, evaluados,
		"ningún provider quedó fuera del baseline: si se limpió la deuda, borrar defaultsConDeudaConocida")
}

// El baseline tiene que poder apretarse. Si alguien resuelve la deuda de google o
// ollama y no saca la entrada, el guard deja de vigilar ese provider en silencio: no es
// un error —bajar deuda siempre se permite— así que avisa sin fallar.
func TestProviderDefaults_BaselineDeDeudaNoQuedaDeMas(t *testing.T) {
	reg := registry.New()

	for _, d := range defaultsDeLosProviders() {
		if !defaultsConDeudaConocida[d.provider] {
			continue
		}
		if _, err := reg.Get(context.Background(), d.provider, d.modelID); err == nil {
			t.Logf("el default de %s (%q) YA está cotizado: sacarlo de defaultsConDeudaConocida para que el guard lo vigile",
				d.provider, d.modelID)
		}
	}
}

// El guard tiene que estar mirando algo. Si New() dejara de inicializar Model —por un
// refactor del constructor— los IDs saldrían vacíos y el test de arriba pasaría o
// fallaría por la razón equivocada.
func TestProviderDefaults_NingunDefaultQuedaVacio(t *testing.T) {
	for _, d := range defaultsDeLosProviders() {
		require.NotEmptyf(t, d.modelID,
			"el default de %s salió vacío: New() dejó de inicializar Model y el guard no está mirando el ID real",
			d.provider)
	}
}
