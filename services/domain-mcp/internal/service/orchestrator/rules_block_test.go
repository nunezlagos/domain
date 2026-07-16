package orchestrator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// DOMAINSERV-3 (Fix C): una platform_policy con body extenso (agent-protocol
// ~17.7KB era la que inflaba el payload de domain_orchestrate) se stubbea con un
// puntero a domain_policy_get; las policies chicas siguen embebidas verbatim.
func TestFormatRulesBlock_PlatformBodyExtenso_SeStubbea(t *testing.T) {
	bigBody := strings.Repeat("P", maxInlinePolicyBody+1)
	platform := []rulePolicy{
		{slug: "agent-protocol", name: "Protocolo de agente", body: bigBody, kind: "convention"},
		{slug: "file-size-limit", name: "Límite de tamaño", body: "archivos < 150 líneas", kind: "convention"},
	}

	out := formatRulesBlock(platform, nil, true)

	assert.NotContains(t, out, bigBody, "el body extenso NO debe embeberse")
	assert.Contains(t, out, `domain_policy_get(slug="agent-protocol")`, "debe apuntar al texto vivo")
	assert.Contains(t, out, "Protocolo de agente", "el título de la policy se conserva")
	assert.Contains(t, out, "archivos < 150 líneas", "las policies chicas siguen verbatim")
}

// DOMAINSERV-24: con stubLarge=false (step 0) la platform_policy extensa se embebe
// verbatim para que el agente reciba el body completo (ej. agent-protocol), sin
// stub. Los steps 1..N siguen stubbeados (test de arriba).
func TestFormatRulesBlock_PlatformBodyExtenso_Step0_SeEmbebeVerbatim(t *testing.T) {
	bigBody := strings.Repeat("P", maxInlinePolicyBody+1)
	platform := []rulePolicy{
		{slug: "agent-protocol", name: "Protocolo de agente", body: bigBody, kind: "convention"},
	}

	out := formatRulesBlock(platform, nil, false)

	assert.Contains(t, out, bigBody, "en el step 0 el body extenso va verbatim")
	assert.NotContains(t, out, "domain_policy_get", "el step 0 no stubbea: el agente recibe el body")
}

// Fix del review DOMAINSERV-3: el stub es SOLO para platform. Una project_policy
// con slug "agent-protocol" (el demo seed crea una con reglas propias) NO debe
// stubbearse aunque sea extensa — sus convenciones específicas van siempre inline.
func TestFormatRulesBlock_ProjectPolicyExtensa_NoSeStubbea(t *testing.T) {
	bigBody := strings.Repeat("X", maxInlinePolicyBody+1)
	project := []rulePolicy{
		{slug: "agent-protocol", name: "Protocolo del proyecto", body: bigBody, kind: "agent_protocol"},
	}

	out := formatRulesBlock(nil, project, true)

	assert.Contains(t, out, bigBody, "las project_policies van SIEMPRE verbatim, nunca stubbeadas")
	assert.NotContains(t, out, "domain_policy_get", "no debe redirigir una project_policy")
}

// El override_platform por kind sigue funcionando tras extraer formatRulesBlock.
func TestFormatRulesBlock_ProjectOverride_OcultaPlatformDelMismoKind(t *testing.T) {
	platform := []rulePolicy{
		{slug: "conventional-commits", name: "Commits plataforma", body: "regla plataforma", kind: "convention"},
	}
	project := []rulePolicy{
		{slug: "commits-proyecto", name: "Commits proyecto", body: "regla proyecto", kind: "convention", override: true},
	}

	out := formatRulesBlock(platform, project, true)

	assert.NotContains(t, out, "regla plataforma", "la platform policy del kind overrideado se oculta")
	assert.Contains(t, out, "regla proyecto", "la project policy que overridea se muestra")
}

// Sin policies el bloque es vacío: no debe emitir el header de reglas.
func TestFormatRulesBlock_SinPolicies_DevuelveVacio(t *testing.T) {
	assert.Empty(t, formatRulesBlock(nil, nil, true))
}
