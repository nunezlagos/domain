package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallClaudePermissions_CreatesAllowWhenMissing(t *testing.T) {
	home := t.TempDir()
	if err := installClaudePermissions(home, "20260101T000000Z"); err != nil {
		t.Fatalf("install: %v", err)
	}
	perms, _ := readSettings(t, home)["permissions"].(map[string]any)
	got := toStringSet(perms["allow"])
	for _, rule := range domainPermissionAllows {
		if !got[rule] {
			t.Fatalf("falta la regla %q en %v", rule, got)
		}
	}
}

func TestInstallClaudePermissions_PreservesUserEntries(t *testing.T) {
	home := t.TempDir()
	path := claudeSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, map[string]any{
		"model": "opus",
		"permissions": map[string]any{
			"allow":       []any{"Bash(ls:*)"},
			"defaultMode": "auto",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := installClaudePermissions(home, "ts"); err != nil {
		t.Fatalf("install: %v", err)
	}
	m := readSettings(t, home)
	if m["model"] != "opus" {
		t.Fatalf("se pisó el setting 'model': %v", m["model"])
	}
	perms, _ := m["permissions"].(map[string]any)
	if perms["defaultMode"] != "auto" {
		t.Fatalf("se pisó defaultMode del usuario: %v", perms["defaultMode"])
	}
	got := toStringSet(perms["allow"])
	if !got["Bash(ls:*)"] {
		t.Fatal("se perdió la regla propia del usuario")
	}
	if !got["mcp__domain-mcp"] {
		t.Fatal("no se agregó la regla mcp__domain-mcp de domain")
	}
}

func TestInstallClaudePermissions_WritesGitDeny(t *testing.T) {
	home := t.TempDir()
	if err := installClaudePermissions(home, "20260101T000000Z"); err != nil {
		t.Fatalf("install: %v", err)
	}
	perms, _ := readSettings(t, home)["permissions"].(map[string]any)
	deny := toStringSet(perms["deny"])
	for _, rule := range domainPermissionDenies {
		if !deny[rule] {
			t.Fatalf("falta la regla deny %q en %v", rule, deny)
		}
	}
	// El deny de git destructivo NO debe bloquear cambio de rama legítimo.
	if deny["Bash(git checkout:*)"] {
		t.Fatal("el deny sobre-bloquea: git checkout <rama> quedaría bloqueado")
	}
}

func TestInstallClaudePermissions_MigratesStaleWriteRules(t *testing.T) {
	home := t.TempDir()
	path := claudeSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, map[string]any{
		"permissions": map[string]any{
			"allow": []any{
				"Write(**/.env*)",
				"Edit(**/.env*)",
				"Write(/home/foo/proyectos/x/**)",
				"Write(**)",
				"Bash(ls:*)",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := installClaudePermissions(home, "ts"); err != nil {
		t.Fatalf("install: %v", err)
	}
	got := toStringSet(readSettings(t, home)["permissions"].(map[string]any)["allow"])
	for rule := range got {
		if strings.HasPrefix(rule, "Write(") {
			t.Fatalf("quedó una regla Write muerta sin migrar: %q", rule)
		}
	}
	if !got["Edit(**/.env*)"] {
		t.Fatal("se perdió Edit(**/.env*)")
	}
	if !got["Edit(/home/foo/proyectos/x/**)"] {
		t.Fatal("no se migró Write(/home/foo/proyectos/x/**) a Edit(...)")
	}
	if !got["Edit(**)"] {
		t.Fatal("no se migró Write(**) a Edit(**)")
	}
	if !got["Bash(ls:*)"] {
		t.Fatal("se perdió la regla propia del usuario Bash(ls:*)")
	}
}

func TestInstallClaudePermissions_Idempotent(t *testing.T) {
	home := t.TempDir()
	if err := installClaudePermissions(home, "ts1"); err != nil {
		t.Fatal(err)
	}
	if err := installClaudePermissions(home, "ts2"); err != nil {
		t.Fatal(err)
	}
	perms, _ := readSettings(t, home)["permissions"].(map[string]any)
	arr, _ := perms["allow"].([]any)
	if len(arr) != len(domainPermissionAllows) {
		t.Fatalf("idempotencia: esperaba %d reglas, hay %d (¿duplicados?)", len(domainPermissionAllows), len(arr))
	}
	matches, _ := filepath.Glob(claudeSettingsPath(home) + ".backup-*")
	if len(matches) != 0 {
		t.Fatalf("corrida idempotente no debe crear backup, encontré %v", matches)
	}
}
