package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// installClaudeSessionStartHook agrega un hook SessionStart a
// ~/.claude/settings.json que ejecuta el script domain-session-start.sh
// (el cual pre-carga bootstrap + code graph + mem context antes del primer
// prompt del usuario, forzando al LLM a tener el contexto disponible).
//
// Idempotente: si el hook ya está, no duplica.
func installClaudeSessionStartHook() {
	home, err := os.UserHomeDir()
	if err != nil {
		warnL("no pude resolver HOME para instalar hook: " + err.Error())
		return
	}
	hookPath := filepath.Join(home, ".local", "share", "domain", "hooks", "domain-session-start.sh")
	if _, err := os.Stat(hookPath); err != nil {
		warnL("hook script no encontrado en " + hookPath + " (re-corré el install canónico para instalarlo)")
		return
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		warnL("no pude leer " + settingsPath + ": " + err.Error())
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		warnL(settingsPath + " corrupto, hook no instalado")
		return
	}

	// verificar si ya está el hook
	hooks, _ := cfg["hooks"].(map[string]any)
	if hooks != nil {
		if arr, ok := hooks["SessionStart"].([]any); ok {
			for _, entry := range arr {
				if m, ok := entry.(map[string]any); ok {
					if hs, ok := m["hooks"].([]any); ok {
						for _, h := range hs {
							if hm, ok := h.(map[string]any); ok {
								if cmd, _ := hm["command"].(string); cmd == hookPath {
									// ya está, no duplicar
									return
								}
							}
						}
					}
				}
			}
		}
	}

	// agregar
	if hooks == nil {
		hooks = map[string]any{}
		cfg["hooks"] = hooks
	}
	newEntry := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookPath,
			},
		},
	}
	hooks["SessionStart"] = append(toArray(hooks["SessionStart"]), newEntry)

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		warnL("marshal settings.json: " + err.Error())
		return
	}
	if err := os.WriteFile(settingsPath, out, 0o600); err != nil {
		warnL("write settings.json: " + err.Error())
		return
	}
	ok("hook SessionStart instalado: " + hookPath)
	ok("→ cada nueva sesión de Claude Code pre-cargará bootstrap + code graph + mem context")
}

func toArray(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{}
}