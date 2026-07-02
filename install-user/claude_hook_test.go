package main

import (
	"os"
	"path/filepath"
	"testing"
)

// prepHookHome arma un HOME temporal con el script de hook presente (requisito
// previo de installClaudeSessionStartHook) y lo fija como HOME del proceso.
// Devuelve el path del settings.json esperado.
func prepHookHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	hookDir := filepath.Join(home, ".local", "share", "domain", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatalf("mkdir hookDir: %v", err)
	}
	hookScript := filepath.Join(hookDir, "domain-session-start.sh")
	if err := os.WriteFile(hookScript, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write hook script: %v", err)
	}
	return claudeSettingsPath(home)
}

// hookInstalled parsea settings.json y reporta si el SessionStart de domain
// está registrado apuntando al script canónico.
func hookInstalled(t *testing.T, settingsPath string) bool {
	t.Helper()
	m, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		t.Fatalf("load settings.json: %v", err)
	}
	hooks, _ := m["hooks"].(map[string]any)
	if hooks == nil {
		return false
	}
	arr, ok := hooks["SessionStart"].([]any)
	if !ok {
		return false
	}
	for _, entry := range arr {
		em, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hs, ok := em["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); filepath.Base(cmd) == "domain-session-start.sh" {
				return true
			}
		}
	}
	return false
}

// Regresión: en una instalación limpia (VPS fresco) ~/.claude/settings.json no
// existe. El hook DEBE crearse igual — de lo contrario el resumen del proyecto
// nunca aparece en el primer mensaje de Claude Code.
func TestInstallHook_CreatesSettingsWhenMissing(t *testing.T) {
	settingsPath := prepHookHome(t)

	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("precondición: settings.json no debería existir aún")
	}

	installClaudeSessionStartHook()

	if !hookInstalled(t, settingsPath) {
		t.Fatal("el hook SessionStart no quedó instalado con settings.json ausente")
	}
}

// Idempotencia: correr el instalador dos veces no duplica el hook.
func TestInstallHook_Idempotent(t *testing.T) {
	settingsPath := prepHookHome(t)

	installClaudeSessionStartHook()
	installClaudeSessionStartHook()

	m, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		t.Fatalf("load settings.json: %v", err)
	}
	hooks := m["hooks"].(map[string]any)
	arr := hooks["SessionStart"].([]any)
	count := 0
	for _, entry := range arr {
		em := entry.(map[string]any)
		hs, _ := em["hooks"].([]any)
		for _, h := range hs {
			hm := h.(map[string]any)
			if cmd, _ := hm["command"].(string); filepath.Base(cmd) == "domain-session-start.sh" {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("hook duplicado: esperaba 1 entrada, hay %d", count)
	}
}

// Preservación: si el usuario ya tiene otros hooks/settings, no se pisan.
func TestInstallHook_PreservesUserSettings(t *testing.T) {
	settingsPath := prepHookHome(t)

	existing := map[string]any{
		"theme": "dark",
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "/usr/bin/mi-hook.sh"},
					},
				},
			},
		},
	}
	if err := writeJSON(settingsPath, existing); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	installClaudeSessionStartHook()

	m, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		t.Fatalf("load settings.json: %v", err)
	}
	if m["theme"] != "dark" {
		t.Errorf("se perdió una setting del usuario: theme=%v", m["theme"])
	}
	if !hookInstalled(t, settingsPath) {
		t.Error("el hook de domain no quedó instalado junto al del usuario")
	}
	// El hook previo del usuario sigue presente.
	found := false
	hooks := m["hooks"].(map[string]any)
	for _, entry := range hooks["SessionStart"].([]any) {
		em := entry.(map[string]any)
		hs, _ := em["hooks"].([]any)
		for _, h := range hs {
			hm := h.(map[string]any)
			if cmd, _ := hm["command"].(string); cmd == "/usr/bin/mi-hook.sh" {
				found = true
			}
		}
	}
	if !found {
		t.Error("se perdió el hook SessionStart previo del usuario")
	}
}
