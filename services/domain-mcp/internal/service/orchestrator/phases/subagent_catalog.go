package phases

import "strings"

// CatalogAgents son los agentes efímeros que install-user distribuye a
// ~/.claude/agents/ y que un SubagentPlan puede nombrar. El server NO sabe
// cuáles tiene instalado el cliente: nombrar uno es una preferencia, nunca un
// requisito, y por eso toda mención va con MarcaFallback.
var CatalogAgents = []string{
	"domain-memory",
	"gherkin-verify",
	"git-archaeology",
	"knowledge-ingest",
	"policy-lookup",
	"repo-scout",
	"ticket-triage",
}

// MarcaFallback es la fórmula que acompaña a cada agente nombrado en un plan.
// El guard de subagent_catalog_test.go cuenta menciones contra marcas: un plan
// que nombre un agente sin ella no se puede ejecutar en un cliente sin catálogo.
const MarcaFallback = "si no está disponible:"

// AgentesNombradosEn devuelve los agentes del catálogo que el plan menciona.
func AgentesNombradosEn(plan string) []string {
	var nombrados []string
	for _, slug := range CatalogAgents {
		if strings.Contains(plan, slug) {
			nombrados = append(nombrados, slug)
		}
	}
	return nombrados
}

// SubagentPlans expone el plan canónico por fase para que el
// AgentTemplatesCatalogSeeder lo siembre a agent_templates.metadata.subagent_plan.
// El plan vive acá y NO en el catálogo del seeder: dos copias del mismo texto se
// desincronizan, que es el defecto que DOMAINSERV-180 vino a cerrar.
// Exponía solo sdd-explore, así que los planes de sdd-4r y sdd-verify existían en el código y
// nunca llegaban a la BD (DOMAINSERV-208). El guard de subagent_plan_registro_test.go cruza
// este map contra lo que cada fase declara, en las dos direcciones.
func SubagentPlans() map[string]string {
	return map[string]string{
		"sdd-explore":    exploreSubagentPlan,
		"sdd-4r":         fourRSubagentPlan,
		"sdd-compliance": complianceSubagentPlan,
		"sdd-verify":     verifySubagentPlan,
		"sdd-onboard":    onboardSubagentPlan,
	}
}
