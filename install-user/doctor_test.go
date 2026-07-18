package main

import (
	"os"
	"path/filepath"
	"testing"
)

// setupHealthyHome arma un HOME temporal con una instalación consistente del
// cliente domain: todos los scripts de hook presentes + registrados, permisos
// allow/deny y las instrucciones globales (domain.md + persona.md). Reutiliza
// las funciones reales del instalador para no divergir del comportamiento.
func setupHealthyHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	// 1. Scripts de hook en disco (requisito de installClaudeSessionStartHook).
	hooksDir := filepath.Join(home, ".local", "share", "domain", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooksDir: %v", err)
	}
	for _, spec := range claudeHooks {
		p := filepath.Join(hooksDir, spec.Script)
		if err := os.WriteFile(p, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	// 2. Registro de hooks + permisos + instrucciones globales.
	installClaudeSessionStartHook()
	if err := installClaudePermissions(home, "ts"); err != nil {
		t.Fatalf("installClaudePermissions: %v", err)
	}
	paths := DetectPlatform().Paths()
	if err := installGlobalInstructions(paths, home, "ts"); err != nil {
		t.Fatalf("installGlobalInstructions: %v", err)
	}
	return home
}

// Caso todo-ok: una instalación consistente pasa el doctor con exit 0.
func TestRunDoctor_AllOK(t *testing.T) {
	home := setupHealthyHome(t)
	if code := runDoctor(home); code != 0 {
		t.Fatalf("esperaba exit 0 en instalación consistente, got %d", code)
	}
}

// Caso falta-hook: si falta el script de un hook, el doctor devuelve exit !=0.
func TestRunDoctor_MissingHookScript(t *testing.T) {
	home := setupHealthyHome(t)
	victim := filepath.Join(home, ".local", "share", "domain", "hooks", claudeHooks[0].Script)
	if err := os.Remove(victim); err != nil {
		t.Fatalf("remove hook script: %v", err)
	}
	if code := runDoctor(home); code == 0 {
		t.Fatal("esperaba exit !=0 al faltar el script de un hook")
	}
}

// Caso falta-permiso: si falta mcp__domain-mcp en permissions.allow, falla.
func TestRunDoctor_MissingAllow(t *testing.T) {
	home := setupHealthyHome(t)
	settingsPath := claudeSettingsPath(home)
	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	perms := cfg["permissions"].(map[string]any)
	// Reemplaza allow por una lista sin la regla de domain.
	perms["allow"] = []any{"Bash(ls:*)"}
	if err := writeJSON(settingsPath, cfg); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if code := runDoctor(home); code == 0 {
		t.Fatal("esperaba exit !=0 al faltar mcp__domain-mcp en allow")
	}
}

// Caso HOME vacío: sin nada instalado, el doctor falla (todo crítico ausente).
func TestRunDoctor_EmptyHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if code := runDoctor(home); code == 0 {
		t.Fatal("esperaba exit !=0 en HOME sin instalación")
	}
}
