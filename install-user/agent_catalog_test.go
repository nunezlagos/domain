package main

import (
	"strings"
	"testing"
)

// DOMAINSERV-137: el installer distribuía UN agente nombrado a mano, así que los agentes
// nuevos del catálogo quedaban en el repo sin llegar al cliente. El catálogo se enumera
// desde templates/agents/ para que agregar un agente sea agregar un archivo.

func TestAgentCatalog_EnumeraCadaAgenteDelDirectorio(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	esperados := []string{"domain-memory", "git-archaeology", "policy-lookup", "repo-scout", "ticket-triage"}
	if len(cat) != len(esperados) {
		t.Fatalf("el catálogo tiene %d agentes, se esperaban %d: %v", len(cat), len(esperados), slugs(cat))
	}
	for i, quiero := range esperados {
		if cat[i].slug != quiero {
			t.Errorf("cat[%d].slug = %q, se esperaba %q (el orden debe ser estable)", i, cat[i].slug, quiero)
		}
	}
}

// El sufijo .opencode.md es una VARIANTE del mismo agente, no un agente aparte: los
// esquemas de frontmatter son incompatibles (ver embed.go). Tratarla como agente propio
// la instalaría en Claude Code, donde su `model: anthropic/...` y su `permission:` no
// significan nada.
func TestAgentCatalog_LaVarianteOpencodeNoEsUnAgentePropio(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		if strings.HasSuffix(a.slug, ".opencode") || strings.Contains(a.slug, ".opencode") {
			t.Errorf("la variante %q se enumeró como agente propio", a.slug)
		}
	}

	dm := buscar(t, cat, "domain-memory")
	if len(dm.opencode) == 0 {
		t.Fatal("domain-memory tiene variante .opencode.md en el repo pero el catálogo no la trae")
	}
	if !strings.Contains(string(dm.opencode), "mode: subagent") {
		t.Error("la variante de opencode no parece ser la de OpenCode")
	}
	if strings.Contains(string(dm.opencode), "disallowedTools:") {
		t.Error("la variante de opencode trae campos de Claude Code")
	}
}

// Un agente sin variante se instala SOLO en Claude Code. Reusar su template para OpenCode
// le daría un frontmatter malformado, que es peor que no instalarlo.
func TestAgentCatalog_AgenteSinVariante_NoInventaUnaParaOpencode(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	// git-archaeology no lleva variante a propósito: su guard es un hook PreToolUse y en
	// OpenCode los hooks son plugins JS GLOBALES, no por agente, así que su restricción no
	// es expresable ahí. Se instala solo donde puede estar acotado.
	ga := buscar(t, cat, "git-archaeology")
	if len(ga.opencode) != 0 {
		t.Errorf("git-archaeology no debe tener variante de OpenCode: su guard no es expresable ahí (%d bytes)", len(ga.opencode))
	}
	if len(ga.claude) == 0 {
		t.Error("git-archaeology debe traer su template de Claude Code")
	}
}

// Dos agentes con el mismo `name` en el mismo directorio hacen que Claude Code cargue uno
// de los dos por orden de lectura del filesystem, sin precedencia documentada. Es un fallo
// silencioso: conviene detectarlo antes de copiar, no después.
func TestAgentCatalog_NombresDuplicados_SonUnError(t *testing.T) {
	cat := []agentTemplate{
		{slug: "uno", claude: []byte("---\nname: repetido\nmodel: haiku\n---\n")},
		{slug: "dos", claude: []byte("---\nname: repetido\nmodel: haiku\n---\n")},
	}

	err := validarNombresUnicos(cat)

	if err == nil {
		t.Fatal("dos agentes con el mismo name deben ser un error")
	}
	if !strings.Contains(err.Error(), "repetido") {
		t.Errorf("el error debe nombrar el name en conflicto, dice: %v", err)
	}
}

func TestAgentCatalog_NombresUnicos_NoEsError(t *testing.T) {
	if err := validarNombresUnicos([]agentTemplate{
		{slug: "uno", claude: []byte("---\nname: uno\n---\n")},
		{slug: "dos", claude: []byte("---\nname: dos\n---\n")},
	}); err != nil {
		t.Errorf("names distintos no deben dar error: %v", err)
	}
}

// El catálogo real tiene que cumplir el invariante, no solo el caso sintético.
func TestAgentCatalog_ElCatalogoRealNoTieneNombresDuplicados(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}
	if err := validarNombresUnicos(cat); err != nil {
		t.Errorf("el catálogo del repo tiene names duplicados: %v", err)
	}
}

// Un agente sin `model` hereda el de la sesión, que es lo que DOMAINSERV-135 vino a
// arreglar: el caso de uso más mecánico corriendo en el modelo más caro.
func TestAgentCatalog_TodoAgenteDeclaraNameYModelo(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		fm := frontmatter(t, a.claude)
		for _, campo := range []string{"name:", "model:"} {
			if !strings.Contains(fm, campo) {
				t.Errorf("%s: falta %q en el frontmatter", a.slug, campo)
			}
		}
		if strings.Contains(fm, "model: fable") {
			t.Errorf("%s: el catálogo no usa fable (modelo-por-clase-de-tarea)", a.slug)
		}
		// El campo acepta TIERS, no IDs de API. Un ID pineado devuelve 404 el día que el
		// modelo se retira —le pasó a claude-3-7-sonnet-20250219 desde el 2026-02-19— y con
		// ~26 agentes serían 26 ediciones por cada lanzamiento. Va sobre TODO el catálogo:
		// el chequeo equivalente en agent_templates_test.go solo cubre domain-memory, y un
		// sabotaje sobre otro agente pasaba sin que nada lo notara.
		if strings.Contains(fm, "model: claude-") || strings.Contains(fm, "model: anthropic/") {
			t.Errorf("%s: model lleva TIER (haiku|sonnet|opus), no un ID pineado", a.slug)
		}
	}
}

func slugs(cat []agentTemplate) []string {
	out := make([]string, 0, len(cat))
	for _, a := range cat {
		out = append(out, a.slug)
	}
	return out
}

func buscar(t *testing.T, cat []agentTemplate, slug string) agentTemplate {
	t.Helper()
	for _, a := range cat {
		if a.slug == slug {
			return a
		}
	}
	t.Fatalf("el catálogo no trae %q; trae %v", slug, slugs(cat))
	return agentTemplate{}
}

// DOMAINSERV-137: los invariantes de la variante valen para TODO el catálogo, no solo para
// domain-memory. Sin esto, la próxima variante puede divergir del original o mezclar
// esquemas sin que nada lo note.
func TestAgentCatalog_TodaVariante_MismoBodyYSinMezclarEsquemas(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	body := func(tpl []byte) string {
		s := string(tpl)
		i := strings.Index(s[4:], "\n---")
		if i < 0 {
			return s
		}
		return strings.TrimSpace(s[4+i+4:])
	}

	conVariante := 0
	for _, a := range cat {
		if len(a.opencode) == 0 {
			continue
		}
		conVariante++

		if body(a.claude) != body(a.opencode) {
			t.Errorf("%s: el body de las dos variantes divergió, es el mismo agente", a.slug)
		}

		fmOpencode := frontmatter(t, a.opencode)
		for _, soloClaude := range []string{"effort:", "disallowedTools:", "tools:", "model: haiku"} {
			if strings.Contains(fmOpencode, soloClaude) {
				t.Errorf("%s: %q es de Claude Code y OpenCode no lo entiende", a.slug, soloClaude)
			}
		}
		if !strings.Contains(fmOpencode, "mode: subagent") {
			t.Errorf("%s: la variante de OpenCode necesita mode: subagent", a.slug)
		}
		if !strings.Contains(fmOpencode, "model: anthropic/") {
			t.Errorf("%s: OpenCode necesita el modelo como provider/model-id", a.slug)
		}

		fmClaude := frontmatter(t, a.claude)
		for _, soloOpencode := range []string{"mode:", "permission:", "temperature:"} {
			if strings.Contains(fmClaude, soloOpencode) {
				t.Errorf("%s: %q es de OpenCode y Claude Code no lo entiende", a.slug, soloOpencode)
			}
		}
	}

	if conVariante < 4 {
		t.Errorf("se esperaban al menos 4 agentes con variante, hay %d", conVariante)
	}
}
