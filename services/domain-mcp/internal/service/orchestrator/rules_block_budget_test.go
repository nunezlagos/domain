package orchestrator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-142: el rulesBlock crecía sin techo. maxInlinePolicyBody acota cada
// policy por separado pero no CUÁNTAS hay, y las project_policies iban verbatim
// siempre: cada policy nueva del proyecto engordaba el SystemPrompt del step 0 —el
// único que viaja inline— hasta que el payload de domain_orchestrate volvió a exceder
// el límite del tool result (58.532 chars medidos en prod).

// Reproduce la forma real medida en prod: 8 project_policies de ~2.5KB (ensayos de
// arquitectura, ~20KB en total) y 24 platform de ~575B (reglas duras, 13.8KB). Ninguna
// individualmente extensa, así que maxInlinePolicyBody no tocaba ni una: el total de
// ~34KB entraba entero al SystemPrompt del step 0.
func projectPoliciesReales() []rulePolicy {
	ps := make([]rulePolicy, 8)
	for i := range ps {
		ps[i] = rulePolicy{
			slug: fmt.Sprintf("policy-proyecto-%d", i),
			name: fmt.Sprintf("Policy de proyecto %d", i),
			body: strings.Repeat("C", 2500),
			kind: "architecture",
		}
	}
	return ps
}

func platformPoliciesReales() []rulePolicy {
	ps := make([]rulePolicy, 24)
	for i := range ps {
		ps[i] = rulePolicy{
			slug: fmt.Sprintf("policy-plataforma-%d", i),
			name: fmt.Sprintf("Policy de plataforma %d", i),
			body: strings.Repeat("P", 575),
			kind: fmt.Sprintf("kind-%d", i),
		}
	}
	return ps
}

func TestFormatRulesBlock_TotalAcotado_AunqueNingunaPolicySeaExtensa(t *testing.T) {
	platform, project := platformPoliciesReales(), projectPoliciesReales()
	crudo := 0
	for _, p := range append(append([]rulePolicy{}, platform...), project...) {
		crudo += len(p.body)
		require.LessOrEqual(t, len(p.body), maxInlinePolicyBody,
			"ninguna es extensa por separado: el cap individual no las tocaría")
	}
	require.Greater(t, crudo, 33000, "el fixture reproduce los ~34KB de bodies medidos en prod")

	out := formatRulesBlock(platform, project, true)

	assert.Less(t, len(out), crudo-15000,
		"el bloque queda acotado por presupuesto, no por cuántas policies haya")
	assert.Contains(t, out, "domain_policy_get", "las excedentes apuntan al texto vivo")
}

// El criterio del reparto: a igual presupuesto entran más reglas accionables si se
// priorizan las chicas. Con los tamaños reales, las 24 reglas duras de plataforma
// (~575B) entran TODAS y lo que cede es el contenido largo de referencia.
func TestFormatRulesBlock_PresupuestoPrioriza_LasChicas(t *testing.T) {
	platform, project := platformPoliciesReales(), projectPoliciesReales()

	out := formatRulesBlock(platform, project, true)

	for _, p := range platform {
		assert.Contains(t, out, p.body,
			"las 24 reglas duras de plataforma caben todas: 13.8KB dentro del presupuesto")
	}
	assert.Contains(t, out, "domain_policy_get",
		"el que cede es el contenido largo, que es para lo que existe domain_policy_get")
}

// Lo que se pierde tiene que ser recuperable: la policy que no entra al presupuesto
// conserva su NOMBRE y el puntero a su body. El cliente no queda sin saber que existe.
func TestFormatRulesBlock_PolicyFueraDePresupuesto_ConservaNombreYPuntero(t *testing.T) {
	project := projectPoliciesReales()

	out := formatRulesBlock(nil, project, true)

	for _, p := range project {
		assert.Contains(t, out, p.name, "toda policy aparece con su nombre, entre o no verbatim")
	}
	assert.Contains(t, out, `domain_policy_get(slug="policy-proyecto-7")`,
		"la última en el reparto queda stubbeada con su puntero")
}

// El reparto por tamaño NO cambia el orden de SALIDA: sigue siendo plataforma y después
// proyecto, para no cambiarle el shape al consumidor (mcp-response-shape-contract).
func TestFormatRulesBlock_ReparteP0rTamano_SinCambiarElOrdenDeSalida(t *testing.T) {
	platform := []rulePolicy{
		{slug: "plat-chica", name: "Plataforma", body: "regla corta de plataforma", kind: "convention"},
	}
	project := []rulePolicy{
		{slug: "proj-grande", name: "Proyecto", body: strings.Repeat("J", maxRulesBlockBody), kind: "architecture"},
	}

	out := formatRulesBlock(platform, project, true)

	assert.Contains(t, out, "regla corta de plataforma", "la chica entra primero al presupuesto")
	assert.NotContains(t, out, strings.Repeat("J", maxRulesBlockBody),
		"la grande ya no cabe después de gastar en la chica")
	assert.Less(t, strings.Index(out, "Plataforma"), strings.Index(out, "Proyecto"),
		"orden de reparto != orden de salida: el shape no le cambia al consumidor")
}

// Una policy que no entra no puede bloquear a las siguientes más chicas: se saltea y el
// reparto sigue. Si no, una sola policy grande vaciaría el bloque entero.
func TestFormatRulesBlock_PolicyQueNoEntra_NoBloqueaALasSiguientes(t *testing.T) {
	project := []rulePolicy{
		{slug: "gorda", name: "Gorda", body: strings.Repeat("G", maxRulesBlockBody+1), kind: "a"},
		{slug: "flaca", name: "Flaca", body: "regla corta y valiosa", kind: "b"},
	}

	out := formatRulesBlock(nil, project, true)

	assert.NotContains(t, out, strings.Repeat("G", maxRulesBlockBody+1), "la gorda no entra")
	assert.Contains(t, out, "regla corta y valiosa", "la flaca sí: el reparto no se corta en la primera que no cabe")
}

// Determinismo: mismas policies en el mismo orden ⇒ mismo bloque. Sin esto, dos
// llamadas al mismo flow podrían entregar reglas distintas.
func TestFormatRulesBlock_Determinista(t *testing.T) {
	platform := []rulePolicy{
		{slug: "p1", name: "P1", body: strings.Repeat("A", 3000), kind: "convention"},
		{slug: "p2", name: "P2", body: strings.Repeat("B", 3000), kind: "security_rule"},
	}
	project := projectPoliciesReales()

	primero := formatRulesBlock(platform, project, true)
	for i := 0; i < 5; i++ {
		assert.Equal(t, primero, formatRulesBlock(platform, project, true))
	}
}

// El presupuesto no puede romper el caso chico: un proyecto con pocas policies las
// sigue recibiendo verbatim, sin un solo stub.
func TestFormatRulesBlock_ProyectoChico_SigueVerbatimSinStubs(t *testing.T) {
	project := []rulePolicy{
		{slug: "a", name: "A", body: "regla A", kind: "convention"},
		{slug: "b", name: "B", body: "regla B", kind: "architecture"},
	}

	out := formatRulesBlock(nil, project, true)

	assert.Contains(t, out, "regla A")
	assert.Contains(t, out, "regla B")
	assert.NotContains(t, out, "domain_policy_get", "sin presión de presupuesto no se stubbea nada")
}
