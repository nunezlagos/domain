package main

import (
	"os"
	"path/filepath"
)

// installClaudeSessionStartHook agrega un hook SessionStart a
// ~/.claude/settings.json que ejecuta el script domain-session-start.sh
// (el cual pre-carga bootstrap + code graph + mem context antes del primer
// prompt del usuario, forzando al LLM a tener el contexto disponible).
//
// Idempotente: si el hook ya está, no duplica. Si settings.json no existe
// (instalación limpia, ej. VPS fresco), lo crea — de lo contrario el resumen
// del proyecto nunca aparecería en el primer mensaje.
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
	settingsPath := claudeSettingsPath(home)
	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		warnL(settingsPath + " corrupto, hook no instalado: " + err.Error())
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

	if _, err := backupIfExists(settingsPath, Timestamp()); err != nil {
		warnL("backup settings.json: " + err.Error())
		return
	}
	if err := writeJSON(settingsPath, cfg); err != nil {
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
