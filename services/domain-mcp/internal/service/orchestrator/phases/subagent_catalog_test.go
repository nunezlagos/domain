package phases

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// catalogoEnDisco es el directorio real que install-user distribuye. El server
// es otro módulo del monorepo, así que el guard se saltea si no está.
const catalogoEnDisco = "../../../../../../install-user/templates/agents"

func agentesEnDisco(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(catalogoEnDisco)
	if err != nil {
		t.Skipf("catálogo no disponible en este checkout: %v", err)
	}
	var slugs []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".opencode.md") {
			continue
		}
		slugs = append(slugs, strings.TrimSuffix(name, ".md"))
	}
	return slugs
}

func TestCatalogAgents_CadaAgenteDeLaLista_TieneTemplateEnInstallUser(t *testing.T) {
	for _, slug := range CatalogAgents {
		if _, err := os.Stat(filepath.Join(catalogoEnDisco, slug+".md")); err != nil {
			if os.IsNotExist(err) {
				t.Errorf("CatalogAgents nombra %q y no hay template en install-user: un plan lo pediría y el cliente no lo tendría", slug)
				continue
			}
			t.Skipf("catálogo no disponible en este checkout: %v", err)
		}
	}
}

func TestCatalogAgents_UnAgenteNuevoEnDisco_NoQuedaFueraDeLaLista(t *testing.T) {
	conocidos := make(map[string]bool, len(CatalogAgents))
	for _, slug := range CatalogAgents {
		conocidos[slug] = true
	}
	for _, slug := range agentesEnDisco(t) {
		if !conocidos[slug] {
			t.Errorf("install-user distribuye %q y CatalogAgents no lo conoce: ningún plan puede nombrarlo", slug)
		}
	}
}

func TestSubagentPlans_CadaAgenteNombrado_EstaEnElCatalogo(t *testing.T) {
	conocidos := make(map[string]bool, len(CatalogAgents))
	for _, slug := range CatalogAgents {
		conocidos[slug] = true
	}
	for phase, plan := range SubagentPlans() {
		for _, slug := range AgentesNombradosEn(plan) {
			if !conocidos[slug] {
				t.Errorf("el plan de %s nombra el agente %q, que no está en CatalogAgents", phase, slug)
			}
		}
	}
}

func TestSubagentPlans_CadaAgenteNombrado_DeclaraSuFallback(t *testing.T) {
	for phase, plan := range SubagentPlans() {
		nombrados := len(AgentesNombradosEn(plan))
		fallbacks := strings.Count(plan, MarcaFallback)
		if nombrados > 0 && fallbacks < nombrados {
			t.Errorf("el plan de %s nombra %d agente(s) del catálogo y declara %d %q: un cliente sin el catálogo no podría ejecutar la fase",
				phase, nombrados, fallbacks, MarcaFallback)
		}
	}
}

// git-archaeology depende de un hook PreToolUse que OpenCode no tiene, así que
// no existe como variante .opencode.md (DOMAINSERV-137).
func TestSubagentPlans_AgenteSinVarianteOpencode_AdvierteElHarness(t *testing.T) {
	for phase, plan := range SubagentPlans() {
		for _, slug := range AgentesNombradosEn(plan) {
			if _, err := os.Stat(filepath.Join(catalogoEnDisco, slug+".opencode.md")); os.IsNotExist(err) {
				if !strings.Contains(plan, "OpenCode") {
					t.Errorf("el plan de %s nombra %q, que no tiene variante OpenCode, sin advertirlo", phase, slug)
				}
			}
		}
	}
}

// sdd-design y sdd-spec prohíben el subagente a propósito: AskUserQuestion no
// existe ahí (REQ-55 issue-55.1).
func TestSubagentPlans_FasesQueInterroganAlUsuario_NoTienenPlan(t *testing.T) {
	for _, phase := range []string{"sdd-design", "sdd-spec"} {
		if plan, ok := SubagentPlans()[phase]; ok && plan != "" {
			t.Errorf("%s no puede declarar SubagentPlan: AskUserQuestion no existe en subagentes", phase)
		}
	}
}

func TestSubagentPlans_ElPlanSembrado_EsElDelHandler(t *testing.T) {
	out, err := NewSDDExploreHandler().Build(t.Context(), Input{RawText: "tarea cualquiera"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if out.SubagentPlan != SubagentPlans()["sdd-explore"] {
		t.Error("el plan que el handler emite y el que el seeder siembra divergieron: son la misma fuente")
	}
}
