package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeOpencodeJSON escribe un opencode.json bajo dir/opencode con el contenido
// dado y devuelve el Paths (linux) apuntando ahí.
func opencodePathsUnder(t *testing.T, home string) Paths {
	t.Helper()
	t.Setenv("HOME", home)
	return Platform{OS: "linux"}.Paths()
}

// Gap1 (DOMAINSERV-102): checkPermissions valida TODAS las reglas de
// domainPermissionAllows, no solo mcp__domain-mcp. Si falta Edit(**) → falla.
func TestCheckPermissions_MissingEditAllow_Fails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settings := claudeSettingsPath(home)
	// allow con mcp__domain-mcp + Read(**) pero SIN Edit(**).
	cfg := map[string]any{"permissions": map[string]any{
		"allow": []any{"mcp__domain-mcp", "Read(**)"},
		"deny":  toAnySlice(domainPermissionDenies),
	}}
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(settings, cfg); err != nil {
		t.Fatal(err)
	}
	if checkPermissions(home) == 0 {
		t.Fatal("checkPermissions debía fallar al faltar Edit(**) en allow (DOMAINSERV-102)")
	}
}

// Gap3 (DOMAINSERV-102): checkOpencodePermission exige el catch-all
// bash["*"]="ask" además de las reglas deny.
func TestCheckOpencodePermission_MissingCatchAll_Fails(t *testing.T) {
	home := t.TempDir()
	paths := opencodePathsUnder(t, home)
	if err := os.MkdirAll(paths.OpencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bash := map[string]any{}
	for _, r := range opencodeGitDenyRules {
		bash[r] = "deny"
	}
	// deliberadamente SIN bash["*"]="ask".
	cfg := map[string]any{"permission": map[string]any{"bash": bash}}
	if err := writeJSON(paths.OpencodeMCP, cfg); err != nil {
		t.Fatal(err)
	}
	if checkOpencodePermission(paths) == 0 {
		t.Fatal("checkOpencodePermission debía fallar sin el catch-all bash[*]=ask")
	}
}

// Gap2 (DOMAINSERV-102): checkOpencodeInstruction falla si OpenCode está
// presente pero falta instructions/domain.md o su referencia.
func TestCheckOpencodeInstruction_Missing_Fails(t *testing.T) {
	home := t.TempDir()
	paths := opencodePathsUnder(t, home)
	if err := os.MkdirAll(paths.OpencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// opencode presente (json existe) pero sin instruction ni referencia.
	if err := writeJSON(paths.OpencodeMCP, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if checkOpencodeInstruction(paths) == 0 {
		t.Fatal("checkOpencodeInstruction debía fallar sin instructions/domain.md")
	}

	// Con el archivo + la referencia en el array → pasa.
	instrDir := filepath.Join(paths.OpencodeDir, "instructions")
	if err := os.MkdirAll(instrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instrDir, "domain.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(paths.OpencodeMCP, map[string]any{"instructions": []any{"instructions/domain.md"}}); err != nil {
		t.Fatal(err)
	}
	if checkOpencodeInstruction(paths) != 0 {
		t.Fatal("checkOpencodeInstruction debía pasar con el archivo + referencia presentes")
	}
}

// E (DOMAINSERV-102): el doctor deriva los paths de Paths (OS-aware), no de
// ~/.config hardcodeado. En Windows el opencode.json vive bajo %APPDATA%; el
// check debe leer ESE archivo (un perm roto ahí → falla, probando que no mira
// el path Unix).
func TestCheckOpencodePermission_WindowsAppData_UsesPaths(t *testing.T) {
	home := t.TempDir()
	appData := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", appData)
	paths := Platform{OS: "windows"}.Paths()

	if err := os.MkdirAll(paths.OpencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// perm block roto (sin reglas) bajo %APPDATA%\opencode.
	if err := writeJSON(paths.OpencodeMCP, map[string]any{"permission": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if checkOpencodePermission(paths) == 0 {
		t.Fatal("checkOpencodePermission debía leer el opencode.json bajo %APPDATA% y fallar (portabilidad Windows)")
	}
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
