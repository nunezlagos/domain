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

	rs := buscar(t, cat, "repo-scout")
	if len(rs.opencode) != 0 {
		t.Errorf("repo-scout no tiene variante .opencode.md en el repo; el catálogo devolvió %d bytes", len(rs.opencode))
	}
	if len(rs.claude) == 0 {
		t.Error("repo-scout debe traer su template de Claude Code")
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
