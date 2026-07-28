package seeds

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-159: el catálogo declaraba temperature entre 0.1 y 0.4 en 13 entradas
// mientras migraba a la familia Claude 5, que responde 400 ante un temperature
// no-default. El provider copia el valor al body cuando es > 0
// (internal/llm/anthropic/provider.go, buildRequest), así que la ruta existe y la
// falla es por invocación, no eventual. Es el mismo defecto que DOMAINSERV-164
// cerró en el catálogo de agentes efímeros: install-user/agent_tier_test.go es el
// molde de estos guards.

// modelosQueRechazanTemperature son los IDs cuya API elimina el parámetro de
// sampling. Opus 4.7 fue el primero de la serie; de ahí en adelante declararlo es
// un 400, no una preferencia de estilo.
var modelosQueRechazanTemperature = map[string]bool{
	"claude-fable-5":  true,
	"claude-opus-5":   true,
	"claude-opus-4-8": true,
	"claude-opus-4-7": true,
	"claude-sonnet-5": true,
}

// modelosRetirados son generaciones que ya tienen reemplazo vigente al mismo
// precio o mejor. Una entrada acá corre más caro y con menos capacidad que su
// reemplazo, sin que nada lo señale.
var modelosRetirados = map[string]string{
	"claude-opus-4-7":   "claude-opus-5",
	"claude-sonnet-4-6": "claude-sonnet-5",
}

// Una entrada que declara temperature junto a un modelo que la rechaza rompe la
// fase en cada invocación: el 400 llega del provider, no del catálogo, así que el
// síntoma aparece lejos de la causa.
func TestAgentTemplateCatalog_NingunaEntradaDeclaraTemperatureEnModeloQueLaRechaza(t *testing.T) {
	var evaluadas int
	for _, e := range AgentTemplateCatalog() {
		if !modelosQueRechazanTemperature[e.Model] {
			continue
		}
		evaluadas++
		if e.Temperature != 0 {
			t.Errorf("la fase %s declara temperature %v con %q: la API responde 400 en cada invocación",
				e.Slug, e.Temperature, e.Model)
		}
	}
	require.NotZero(t, evaluadas,
		"ninguna entrada del catálogo usa un modelo que rechace temperature: el guard no está mirando nada")
}

// El catálogo es el lado server de la policy modelo-por-clase-de-tarea. Una
// generación retirada no falla: corre, cuesta más y rinde menos, y nadie se entera.
func TestAgentTemplateCatalog_NingunaEntradaSigueEnGeneracionRetirada(t *testing.T) {
	entradas := AgentTemplateCatalog()
	require.NotEmpty(t, entradas, "catálogo vacío: el guard no está mirando nada")

	for _, e := range entradas {
		if reemplazo, retirado := modelosRetirados[e.Model]; retirado {
			t.Errorf("la fase %s corre en %q, retirado: el reemplazo vigente es %q",
				e.Slug, e.Model, reemplazo)
		}
	}
}

// El bump de Version() es lo que habilita el re-seed: sin él, applied_version >=
// Version() y las filas viejas conservan model y temperature de la generación
// anterior, o sea que el trabajo queda invisible en producción.
func TestAgentTemplatesSeedVersion_CubreLaMigracionAClaude5(t *testing.T) {
	require.GreaterOrEqual(t, agentTemplatesSeedVersion, 19,
		"migrar el catálogo a Claude 5 exige bump de agentTemplatesSeedVersion; sin él el re-seed no corre y las filas siguen con temperature")
}

// El campo Temperature NO se borra del struct aunque ninguna entrada Claude 5 lo
// use: sdd-archive se queda en haiku 4.5, que sí lo acepta. Sin campo, el guard de
// arriba no tendría objeto — nadie podría declararlo mal y el test no verificaría
// nada. Este test fija esa razón para que un "limpiar el struct" futuro falle acá.
func TestAgentTemplateCatalog_ElCampoTemperatureConservaUnUsuarioLegitimo(t *testing.T) {
	var conTemperature int
	for _, e := range AgentTemplateCatalog() {
		if e.Temperature == 0 {
			continue
		}
		conTemperature++
		require.False(t, modelosQueRechazanTemperature[e.Model],
			"la fase %s declara temperature con %q, que la rechaza", e.Slug, e.Model)
	}
	require.NotZero(t, conTemperature,
		"ninguna entrada declara temperature: el campo quedó sin usuarios y el guard de temperature no tiene objeto que vigilar")
}
