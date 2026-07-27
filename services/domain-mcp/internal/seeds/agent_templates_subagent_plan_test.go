package seeds

import (
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
