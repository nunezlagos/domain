package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-135: el subagente domain-memory heredaba el modelo de la sesión
// (opus[1m] + effortLevel high) y TODO el pool de tools para hacer un recall que es
// búsqueda estructurada con formato de salida fijo. Al acotarlo apareció el problema de
// fondo: el MISMO archivo se instalaba en Claude Code y —por symlink— en OpenCode, y sus
// esquemas de frontmatter son incompatibles. De ahí el split en dos templates.

// DOMAINSERV-137: los templates salen del catálogo, no de un embed por nombre. Así estos
// tests siguen la misma fuente que el installer y no pueden divergir de ella.
func tplDe(t *testing.T, slug string) agentTemplate {
	t.Helper()
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}
	for _, a := range cat {
		if a.slug == slug {
			return a
		}
	}
	t.Fatalf("el catálogo no trae %q", slug)
	return agentTemplate{}
}

// frontmatter devuelve el bloque YAML entre los dos '---' de un template.
func frontmatter(t *testing.T, tpl []byte) string {
	t.Helper()
	s := string(tpl)
	if !strings.HasPrefix(s, "---\n") {
		t.Fatalf("el template no arranca con frontmatter: %.40q", s)
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		t.Fatal("frontmatter sin cierre '---'")
	}
	return s[4 : 4+end]
}

func TestAgentTemplate_ClaudeCode_AcotaModeloEffortYTools(t *testing.T) {
	fm := frontmatter(t, tplDe(t, "domain-memory").claude)

	for _, campo := range []string{"name:", "description:", "model:", "effort:", "tools:"} {
		if !strings.Contains(fm, campo) {
			t.Errorf("falta %q en el frontmatter de Claude Code", campo)
		}
	}
	// sin `name` la identidad dependería del nombre de archivo; la doc es explícita en
	// que viene solo de este campo
	if !strings.Contains(fm, "name: domain-memory") {
		t.Error("name debe ser domain-memory")
	}
	// Decisión del usuario 2026-07-26: sonnet para búsquedas y tareas normales; opus queda
	// para el orquestador y la implementación puntual. Sube desde haiku porque la medición A/B
	// mostró que el modo de falla del agente único era de CAPACIDAD —estimar referencias bajo
	// presión de contexto, 6 de 6 números de línea errados— y el tier ataca la causa.
	if !strings.Contains(fm, "model: sonnet") {
		t.Error("model debe ser el tier sonnet, no heredado de la sesión")
	}
	// El campo acepta TIERS, no IDs. Un ID pineado devuelve 404 el día que el modelo se
	// retira: le pasó a claude-3-7-sonnet-20250219 desde el 2026-02-19. Con el tier, un
	// modelo nuevo lo heredan los 5 agentes sin editar un archivo.
	if strings.Contains(fm, "model: claude-") || strings.Contains(fm, "model: anthropic/") {
		t.Error("el frontmatter de Claude Code lleva TIER, no un provider/model-id pineado")
	}
	if !strings.Contains(fm, "effort: low") {
		t.Error("effort debe ser low: es búsqueda con formato de salida fijo")
	}
}

// El agente se declara read-only. Antes eso era una frase en el body ("No mem_save /
// knowledge_save"), que no es un constraint: tenía capacidad de escritura real.
func TestAgentTemplate_ClaudeCode_SinToolsDeEscritura(t *testing.T) {
	fm := frontmatter(t, tplDe(t, "domain-memory").claude)

	for _, escritura := range []string{
		"domain_mem_save", "domain_knowledge_save", "Write", "Edit", "NotebookEdit",
	} {
		linea := lineaDe(fm, "tools:")
		if strings.Contains(linea, escritura) {
			t.Errorf("la allowlist de tools no puede incluir %q", escritura)
		}
	}
	if !strings.Contains(fm, "disallowedTools:") {
		t.Error("falta disallowedTools: documenta la intención read-only además de la allowlist")
	}
}

// ToolSearch es obligatorio: `env` está vacío en settings.json, así que los schemas MCP
// están deferred. Sin ToolSearch el agente no puede cargar el schema de domain_mem_search
// y no puede invocar nada — se queda mudo sin error visible.
func TestAgentTemplate_ClaudeCode_IncluyeToolSearch(t *testing.T) {
	fm := frontmatter(t, tplDe(t, "domain-memory").claude)

	if !strings.Contains(lineaDe(fm, "tools:"), "ToolSearch") {
		t.Error("ToolSearch debe estar en la allowlist: los schemas MCP están deferred")
	}
}

// Los nombres de las tools MCP tienen que ser los EXACTOS: si una entrada de `tools` no
// resuelve, el subagente falla al spawnear con un error nombrando las entradas.
func TestAgentTemplate_ClaudeCode_NombresMCPCompletos(t *testing.T) {
	linea := lineaDe(frontmatter(t, tplDe(t, "domain-memory").claude), "tools:")

	for _, tool := range []string{
		"mcp__domain-mcp__domain_mem_search",
		"mcp__domain-mcp__domain_mem_get_observation",
		"mcp__domain-mcp__domain_knowledge_search",
		"mcp__domain-mcp__domain_timeline",
	} {
		if !strings.Contains(linea, tool) {
			t.Errorf("falta la tool %q con su prefijo mcp__ completo", tool)
		}
	}
}

// OpenCode tiene OTRO esquema: model en forma provider/model-id y restricciones por
// `permission`, no por `tools`. Un `model: haiku` pelado es un valor MALFORMADO de un
// campo que OpenCode sí conoce — no un campo desconocido que pueda ignorar.
func TestAgentTemplate_Opencode_UsaSuPropioEsquema(t *testing.T) {
	fm := frontmatter(t, tplDe(t, "domain-memory").opencode)

	if !strings.Contains(fm, "mode: subagent") {
		t.Error("falta mode: subagent")
	}
	if !strings.Contains(fm, "model: anthropic/") {
		t.Error("model debe ir en forma provider/model-id")
	}
	if !strings.Contains(fm, "permission:") {
		t.Error("OpenCode restringe con permission:, no con tools:")
	}
}

// El corazón del split: ninguna clave exclusiva de Claude Code puede viajar en el
// template de OpenCode, ni al revés.
func TestAgentTemplate_Opencode_SinClavesDeClaudeCode(t *testing.T) {
	fm := frontmatter(t, tplDe(t, "domain-memory").opencode)

	for _, soloClaude := range []string{"effort:", "disallowedTools:", "tools:", "model: haiku"} {
		if strings.Contains(fm, soloClaude) {
			t.Errorf("%q es de Claude Code y no lo entiende OpenCode", soloClaude)
		}
	}
}

func TestAgentTemplate_ClaudeCode_SinClavesDeOpencode(t *testing.T) {
	fm := frontmatter(t, tplDe(t, "domain-memory").claude)

	for _, soloOpencode := range []string{"mode:", "permission:", "temperature:"} {
		if strings.Contains(fm, soloOpencode) {
			t.Errorf("%q es de OpenCode y no lo entiende Claude Code", soloOpencode)
		}
	}
}

// Los dos templates describen al MISMO agente: el body (procedimiento y formato de
// retorno) no puede divergir, o el recall devuelve formatos distintos según el cliente.
func TestAgentTemplate_AmbosClientes_MismoBody(t *testing.T) {
	body := func(tpl []byte) string {
		s := string(tpl)
		i := strings.Index(s[4:], "\n---")
		return strings.TrimSpace(s[4+i+4:])
	}

	if body(tplDe(t, "domain-memory").claude) != body(tplDe(t, "domain-memory").opencode) {
		t.Error("el body de los dos templates divergió: es el mismo agente")
	}
}

// Regresión del bug de fondo: el agente de OpenCode era un SYMLINK al de Claude Code, o
// sea un archivo con dos esquemas incompatibles. Ahora es un archivo propio.
func TestInstallGlobalAssets_AgenteOpencode_NoEsSymlinkDelDeClaude(t *testing.T) {
	tmp := t.TempDir()
	paths := Paths{
		GlobalSkillPath:   filepath.Join(tmp, "claude", "skills", "domain", "SKILL.md"),
		GlobalAgentsDir:   filepath.Join(tmp, "claude", "agents"),
		OpencodeSkillsLn:  filepath.Join(tmp, "opencode", "skills", "domain"),
		OpencodeAgentsDir: filepath.Join(tmp, "opencode", "agents"),
	}
	agenteOpencode := filepath.Join(paths.OpencodeAgentsDir, "domain-memory.md")

	if _, err := installGlobalAssets(paths); err != nil {
		t.Fatalf("installGlobalAssets: %v", err)
	}
	if err := linkOpencodeToGlobal(paths, "linux"); err != nil {
		t.Fatalf("linkOpencodeToGlobal: %v", err)
	}

	fi, err := os.Lstat(agenteOpencode)
	if err != nil {
		t.Fatalf("el agente de OpenCode no se escribió: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("el agente de OpenCode es un symlink al de Claude Code: un archivo no puede satisfacer los dos esquemas")
	}

	b, err := os.ReadFile(agenteOpencode)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "mode: subagent") {
		t.Error("el archivo instalado en OpenCode no es el template de OpenCode")
	}
	if strings.Contains(string(b), "effort:") {
		t.Error("el archivo instalado en OpenCode trae frontmatter de Claude Code")
	}

	// el SKILL sí sigue siendo symlink: su formato es compatible entre clientes
	if fi, err := os.Lstat(paths.OpencodeSkillsLn); err != nil {
		t.Fatalf("skill de OpenCode: %v", err)
	} else if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("el skill dejó de ser symlink sin razón: solo el agente necesitaba split")
	}
}

// En Windows no hay symlinks, así que el skill se copia. El agente de OpenCode debe
// seguir siendo su propio template, no una copia del de Claude Code.
func TestInstallGlobalAssets_Windows_AgenteOpencodeEsSuPropioTemplate(t *testing.T) {
	tmp := t.TempDir()
	paths := Paths{
		GlobalSkillPath:   filepath.Join(tmp, "claude", "skills", "domain", "SKILL.md"),
		GlobalAgentsDir:   filepath.Join(tmp, "claude", "agents"),
		OpencodeSkillsLn:  filepath.Join(tmp, "opencode", "skills", "domain"),
		OpencodeAgentsDir: filepath.Join(tmp, "opencode", "agents"),
	}

	if _, err := installGlobalAssets(paths); err != nil {
		t.Fatalf("installGlobalAssets: %v", err)
	}
	if err := linkOpencodeToGlobal(paths, "windows"); err != nil {
		t.Fatalf("linkOpencodeToGlobal: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(paths.OpencodeAgentsDir, "domain-memory.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "mode: subagent") {
		t.Error("en Windows también debe instalarse el template de OpenCode")
	}
}

// lineaDe devuelve la línea del frontmatter que arranca con el prefijo dado, incluyendo
// las líneas de continuación indentadas (para un `tools:` multilínea).
func lineaDe(fm, prefijo string) string {
	lineas := strings.Split(fm, "\n")
	var out []string
	capturando := false
	for _, l := range lineas {
		if strings.HasPrefix(l, prefijo) {
			capturando = true
			out = append(out, l)
			continue
		}
		if capturando {
			if strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t") || strings.HasPrefix(l, "-") {
				out = append(out, l)
				continue
			}
			break
		}
	}
	return strings.Join(out, "\n")
}
