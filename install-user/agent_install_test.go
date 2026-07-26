package main

import (
	"os"
	"path/filepath"
	"testing"
)

// DOMAINSERV-137: installGlobalAssets escribía UN agente por nombre. Estos tests fijan el
// contrato de instalar el catálogo completo.

func pathsDePrueba(t *testing.T) Paths {
	t.Helper()
	tmp := t.TempDir()
	return Paths{
		GlobalSkillPath:   filepath.Join(tmp, "claude", "skills", "domain", "SKILL.md"),
		GlobalAgentsDir:   filepath.Join(tmp, "claude", "agents"),
		OpencodeDir:       filepath.Join(tmp, "opencode"),
		OpencodeAgentsDir: filepath.Join(tmp, "opencode", "agents"),
	}
}

func catalogoDePrueba() []agentTemplate {
	return []agentTemplate{
		{slug: "con-variante", claude: []byte("---\nname: con-variante\nmodel: haiku\n---\nclaude\n"),
			opencode: []byte("---\nmode: subagent\nmodel: anthropic/claude-haiku-4-5\n---\nopencode\n")},
		{slug: "sin-variante", claude: []byte("---\nname: sin-variante\nmodel: haiku\n---\nclaude\n")},
	}
}

func TestInstalarAgentes_EscribeTodoElCatalogoEnClaudeCode(t *testing.T) {
	paths := pathsDePrueba(t)

	res, err := instalarAgentes(paths, catalogoDePrueba())
	if err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	if len(res.instalados) != 2 {
		t.Errorf("instalados = %v, se esperaban los 2 del catálogo", res.instalados)
	}
	for _, slug := range []string{"con-variante", "sin-variante"} {
		b, err := os.ReadFile(filepath.Join(paths.GlobalAgentsDir, slug+".md"))
		if err != nil {
			t.Fatalf("%s no quedó instalado: %v", slug, err)
		}
		if string(b) != "---\nname: "+slug+"\nmodel: haiku\n---\nclaude\n" {
			t.Errorf("%s: se instaló contenido inesperado: %q", slug, b)
		}
	}
}

// Un agente sin variante NO se instala en OpenCode: su frontmatter de Claude Code sería
// malformado ahí (ver embed.go). Y la omisión se reporta, porque una ausencia silenciosa
// se lee como cobertura.
func TestInstalarAgentes_SoloLosQueTienenVarianteVanAOpencode(t *testing.T) {
	paths := pathsDePrueba(t)

	res, err := instalarAgentes(paths, catalogoDePrueba())
	if err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(paths.OpencodeAgentsDir, "con-variante.md"))
	if err != nil {
		t.Fatalf("con-variante debía instalarse en OpenCode: %v", err)
	}
	if string(b) != "---\nmode: subagent\nmodel: anthropic/claude-haiku-4-5\n---\nopencode\n" {
		t.Errorf("OpenCode recibió el template equivocado: %q", b)
	}

	if _, err := os.Stat(filepath.Join(paths.OpencodeAgentsDir, "sin-variante.md")); !os.IsNotExist(err) {
		t.Error("sin-variante NO debe instalarse en OpenCode: su frontmatter de Claude Code es malformado ahí")
	}
	if len(res.omitidosOpencode) != 1 || res.omitidosOpencode[0] != "sin-variante" {
		t.Errorf("la omisión debe reportarse, omitidosOpencode = %v", res.omitidosOpencode)
	}
}

// Sin OpenCode instalado no hay nada que escribir, y eso no es un error.
func TestInstalarAgentes_SinOpencode_NoFalla(t *testing.T) {
	paths := pathsDePrueba(t)
	paths.OpencodeAgentsDir = ""

	res, err := instalarAgentes(paths, catalogoDePrueba())
	if err != nil {
		t.Fatalf("instalarAgentes sin OpenCode: %v", err)
	}
	if len(res.instalados) != 2 {
		t.Errorf("Claude Code debe recibir el catálogo igual: %v", res.instalados)
	}
	if len(res.opencode) != 0 {
		t.Errorf("sin OpenCode no se instala nada ahí: %v", res.opencode)
	}
}

func TestInstalarAgentes_DosCorridas_MismoEstadoYSinConflictos(t *testing.T) {
	paths := pathsDePrueba(t)
	cat := catalogoDePrueba()

	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	res, err := instalarAgentes(paths, cat)
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}

	if len(res.conflictos) != 0 {
		t.Errorf("re-instalar lo mismo no es un conflicto: %v", res.conflictos)
	}
	entries, err := os.ReadDir(paths.GlobalAgentsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("la segunda corrida dejó %d archivos, se esperaban 2 (sin duplicados)", len(entries))
	}
}

// Una instalación previa dejaba un SYMLINK en el path de OpenCode. WriteFile escribiría A
// TRAVÉS del symlink y pisaría el template de Claude Code con el de OpenCode.
func TestInstalarAgentes_SymlinkPrevioEnOpencode_NoSeEscribeATraves(t *testing.T) {
	paths := pathsDePrueba(t)
	if err := os.MkdirAll(paths.GlobalAgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.OpencodeAgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	destino := filepath.Join(paths.GlobalAgentsDir, "con-variante.md")
	if err := os.WriteFile(destino, []byte("contenido de claude code\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(destino, filepath.Join(paths.OpencodeAgentsDir, "con-variante.md")); err != nil {
		t.Skipf("symlinks no disponibles: %v", err)
	}

	if _, err := instalarAgentes(paths, catalogoDePrueba()); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	b, err := os.ReadFile(destino)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "---\nmode: subagent\nmodel: anthropic/claude-haiku-4-5\n---\nopencode\n" {
		t.Error("se escribió a través del symlink: el template de OpenCode pisó el de Claude Code")
	}
}

// Desinstalación simétrica: se remueve exactamente lo del catálogo, y lo que el usuario
// haya puesto en el mismo directorio queda intacto.
func TestDesinstalarAgentes_RemueveLoDelCatalogoYNoLoAjeno(t *testing.T) {
	paths := pathsDePrueba(t)
	cat := catalogoDePrueba()
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}
	ajeno := filepath.Join(paths.GlobalAgentsDir, "mi-agente-propio.md")
	if err := os.WriteFile(ajeno, []byte("mío\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	desinstalarAgentes(paths, cat)

	for _, slug := range []string{"con-variante", "sin-variante"} {
		if _, err := os.Stat(filepath.Join(paths.GlobalAgentsDir, slug+".md")); !os.IsNotExist(err) {
			t.Errorf("%s debía removerse", slug)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.OpencodeAgentsDir, "con-variante.md")); !os.IsNotExist(err) {
		t.Error("el agente de OpenCode debía removerse")
	}
	if _, err := os.Stat(ajeno); err != nil {
		t.Error("un agente ajeno al catálogo NO se toca en la desinstalación")
	}
}
