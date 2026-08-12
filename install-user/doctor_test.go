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
	homeAislado(t, home)

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
	// DOMAINSERV-267: domain-hooks-lib.sh no está en claudeHooks —no es un hook registrado—
	// así que este fixture, que representa una instalación SANA, la venía omitiendo. Una
	// instalación sin ella no es sana: todos los hooks la cargan con `. "$LIB"` y fallan en su
	// primera línea útil. El doctor ahora la audita, y el fixture tiene que reflejarlo.
	if err := os.WriteFile(filepath.Join(hooksDir, "domain-hooks-lib.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write domain-hooks-lib.sh: %v", err)
	}

	// 2. Registro de hooks + permisos + instrucciones globales.
	installClaudeSessionStartHook(perfilDefault(home))
	if err := installClaudePermissions(perfilDefault(home), "ts"); err != nil {
		t.Fatalf("installClaudePermissions: %v", err)
	}
	if err := installGlobalInstructions(perfilDefault(home), "ts"); err != nil {
		t.Fatalf("installGlobalInstructions: %v", err)
	}
	if err := installClaudeMdExcludes(perfilDefault(home), "ts", false); err != nil {
		t.Fatalf("installClaudeMdExcludes: %v", err)
	}
	// entry MCP con la misma forma que escribe el instalador (headers.Authorization).
	claudeJSON := filepath.Join(home, ".claude.json")
	if err := writeJSON(claudeJSON, map[string]any{
		"mcpServers": map[string]any{
			"domain-mcp": map[string]any{
				"type":    "http",
				"url":     "https://vps.example/mcp",
				"headers": map[string]any{"Authorization": "Bearer test-key"},
			},
		},
	}); err != nil {
		t.Fatalf("write .claude.json: %v", err)
	}

	// 3. Catálogo de agentes. DOMAINSERV-137: el doctor ahora también lo verifica, así que un
	// HOME sin agentes ya no es una instalación consistente.
	//
	// OpencodeAgentsDir se vacía a propósito: este fixture describe una máquina SIN OpenCode
	// —es lo que describía antes, con todos los checks de OpenCode omitidos— y crear
	// ~/.config/opencode/agents/ los activaría en cascada.
	paths := DetectPlatform().Paths()
	paths.OpencodeAgentsDir = ""
	if _, err := installGlobalAssets(paths); err != nil {
		t.Fatalf("installGlobalAssets: %v", err)
	}
	return home
}

// Caso todo-ok: una instalación consistente pasa el doctor con exit 0.
func TestRunDoctor_AllOK(t *testing.T) {
	setupHealthyHome(t)
	if code := runDoctor(DetectPlatform()); code != 0 {
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
	if code := runDoctor(DetectPlatform()); code == 0 {
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
	if code := runDoctor(DetectPlatform()); code == 0 {
		t.Fatal("esperaba exit !=0 al faltar mcp__domain-mcp en allow")
	}
}

// Caso HOME vacío: sin nada instalado, el doctor falla (todo crítico ausente).
func TestRunDoctor_EmptyHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if code := runDoctor(DetectPlatform()); code == 0 {
		t.Fatal("esperaba exit !=0 en HOME sin instalación")
	}
}
