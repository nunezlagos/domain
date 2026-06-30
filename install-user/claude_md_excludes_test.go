package main

import (
	"os"
	"path/filepath"
	"testing"
)

func readSettings(t *testing.T, home string) map[string]any {
	t.Helper()
	m, err := loadOrEmptyJSON(claudeSettingsPath(home))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	return m
}

func toStringSet(v any) map[string]bool {
	out := map[string]bool{}
	if arr, ok := v.([]any); ok {
		for _, e := range arr {
			if s, ok := e.(string); ok {
				out[s] = true
			}
		}
	}
	return out
}

func TestInstallClaudeMdExcludes_WritesAllGlobs(t *testing.T) {
	home := t.TempDir()
	if err := installClaudeMdExcludes(home, "20260101T000000Z", false); err != nil {
		t.Fatalf("install: %v", err)
	}
	got := toStringSet(readSettings(t, home)["claudeMdExcludes"])
	for _, g := range localInstructionGlobs {
		if !got[g] {
			t.Fatalf("falta el glob %q en %v", g, got)
		}
	}
}

func TestInstallClaudeMdExcludes_KeepLocalIsNoOp(t *testing.T) {
	home := t.TempDir()
	if err := installClaudeMdExcludes(home, "ts", true); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(claudeSettingsPath(home)); !os.IsNotExist(err) {
		t.Fatal("keepLocal=true no debe crear settings.json")
	}
}

func TestInstallClaudeMdExcludes_PreservesUserEntries(t *testing.T) {
	home := t.TempDir()
	path := claudeSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, map[string]any{
		"model":            "opus",
		"claudeMdExcludes": []any{"**/secret.md"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := installClaudeMdExcludes(home, "ts", false); err != nil {
		t.Fatalf("install: %v", err)
	}
	m := readSettings(t, home)
	if m["model"] != "opus" {
		t.Fatalf("se pisó el setting 'model' del usuario: %v", m["model"])
	}
	got := toStringSet(m["claudeMdExcludes"])
	if !got["**/secret.md"] {
		t.Fatal("se perdió el exclude propio del usuario")
	}
	if !got["**/AGENTS.md"] {
		t.Fatal("no se agregaron los globs de domain")
	}
}

func TestUpsertStringInArray_PreservesScalarValue(t *testing.T) {
	// Si claudeMdExcludes viniera como string suelto (no array), no debe perderse.
	m := map[string]any{"claudeMdExcludes": "**/secret.md"}
	if !upsertStringInArray(m, "claudeMdExcludes", "**/AGENTS.md") {
		t.Fatal("esperaba mutación")
	}
	got := toStringSet(m["claudeMdExcludes"])
	if !got["**/secret.md"] {
		t.Fatal("se perdió el valor escalar previo del usuario")
	}
	if !got["**/AGENTS.md"] {
		t.Fatal("no se agregó el nuevo valor")
	}
}

func TestInstallClaudeMdExcludes_Idempotent(t *testing.T) {
	home := t.TempDir()
	if err := installClaudeMdExcludes(home, "ts1", false); err != nil {
		t.Fatal(err)
	}
	if err := installClaudeMdExcludes(home, "ts2", false); err != nil {
		t.Fatal(err)
	}
	arr, _ := readSettings(t, home)["claudeMdExcludes"].([]any)
	if len(arr) != len(localInstructionGlobs) {
		t.Fatalf("idempotencia: esperaba %d globs, hay %d (¿duplicados?)", len(localInstructionGlobs), len(arr))
	}
	matches, _ := filepath.Glob(claudeSettingsPath(home) + ".backup-*")
	if len(matches) != 0 {
		t.Fatalf("corrida idempotente no debe crear backup, encontré %v", matches)
	}
}
