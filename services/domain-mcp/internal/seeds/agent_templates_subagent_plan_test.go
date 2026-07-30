package seeds

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// DOMAINSERV-180: el plan de subagentes de una fase se siembra a
// agent_templates.metadata.subagent_plan tomándolo del handler. Si la fase no
// tiene entrada en el catálogo, el plan no se siembra y nadie se entera: el
// override queda vacío y gana el default del código, que es justo lo que este
// ticket vino a conectar.
func TestSubagentPlans_TodaFaseConPlan_TieneEntradaEnElCatalogo(t *testing.T) {
	planes := phases.SubagentPlans()
	require.NotEmpty(t, planes, "sin planes el test no cubre nada")

	enCatalogo := make(map[string]bool)
	for _, e := range AgentTemplateCatalog() {
		enCatalogo[e.Slug] = true
	}

	for slug := range planes {
		require.True(t, enCatalogo[slug],
			"la fase %q declara SubagentPlan y no tiene agent_template en el catálogo: el plan nunca llega a metadata.subagent_plan", slug)
	}
}

// El bump de Version() es lo que habilita el re-seed: sin él, applied_version >=
// Version() y el catálogo nuevo se skippea en silencio (mismo modo de falla que
// DOMAINSERV-179 en el seeder de first-response).
func TestAgentTemplatesSeedVersion_CubreElSubagentPlan(t *testing.T) {
	require.GreaterOrEqual(t, agentTemplatesSeedVersion, 18,
		"sembrar subagent_plan exige bump de agentTemplatesSeedVersion; sin él el re-seed no corre")
}

// DOMAINSERV-161: el fan-out de domain_policy_get en sdd-review vive en el prompt
// seedeado. seeds.go skippea el seeder si applied_version >= Version(), así que sin bump
// el prompt VIEJO sigue gobernando el gate en producción — y el síntoma es
// indistinguible del éxito. Este test ata las dos cosas.
func TestAgentTemplates_FanOutDePolicyGet_ExigeBumpDeVersion(t *testing.T) {
	var reviewPrompt string
	for _, tpl := range AgentTemplateCatalog() {
		if tpl.Slug == "sdd-review" {
			reviewPrompt = tpl.SystemPrompt
			break
		}
	}
	require.NotEmpty(t, reviewPrompt, "sdd-review debe estar en el catálogo")

	require.Contains(t, reviewPrompt, "domain_policy_get",
		"el prompt del gate debe ordenar el fan-out: los listados ya no traen body_md")
	require.Contains(t, reviewPrompt, "policies_checked",
		"el prompt debe declarar que un compliant con policies_checked en 0 se rechaza")
	require.GreaterOrEqual(t, agentTemplatesSeedVersion, 21,
		"el fan-out en el prompt seedeado exige bump a 21: sin él el seeder se skippea y el prompt viejo sigue en BD")
}


// MUST-7 de DOMAINSERV-161: mem_search pasa a devolver 200 caracteres, así que la policy
// que re-hidrata tras compactación tiene que pedir la observación completa por id. Sin
// esto el agente lee el 4% del resumen y cree que retomó el hilo.
// Lee el fuente porque el catálogo vive inline en Run(), igual que voseo_guard_test.
func TestPlatformPolicies_ContextPreservation_ReHidrataConFanOut(t *testing.T) {
	b, err := os.ReadFile("platform_policies_seeder.go")
	require.NoError(t, err)
	fuente := string(b)

	idx := strings.Index(fuente, `Slug:       "context-preservation"`)
	require.Greater(t, idx, 0, "context-preservation debe estar en el catálogo")
	// El corte va en la entrada SIGUIENTE del catálogo. Dos trampas: SourceFile precede a
	// BodyMD, así que cortar ahí dejaría el body afuera; y buscar el marcador desde la
	// posición 0 encuentra el propio Slug de esta entrada, con lo cual no cortaría nada y
	// el test terminaría buscando en el resto del archivo — verde por la razón equivocada.
	const marcador = `Slug:       "`
	bloque := fuente[idx+len(marcador):]
	fin := strings.Index(bloque, marcador)
	require.Greater(t, fin, 0, "no se pudo acotar la ventana a la entrada de context-preservation")
	bloque = bloque[:fin]

	require.Contains(t, bloque, "domain_mem_get_observation",
		"la policy debe ordenar el fan-out por id: el snippet de 200 no alcanza para re-hidratar")
	require.Contains(t, bloque, "content_len",
		"la policy debe nombrar el campo que revela que hay cuerpo sin leer")
	require.GreaterOrEqual(t, (&PlatformPoliciesSeeder{}).Version(), 26,
		"editar el body de una platform policy exige bump o el seeder se skippea y la versión vieja sigue en BD")
}
