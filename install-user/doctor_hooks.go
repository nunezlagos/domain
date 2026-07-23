package main

import (
	"fmt"
	"path/filepath"
)

// checkHooks verifica cada hook de claudeHooks: script presente en disco Y
// registrado en settings.json. Devuelve la cantidad de hooks con problemas.
func checkHooks(home string) int {
	step("Hooks (settings.json + scripts)")
	settingsPath := claudeSettingsPath(home)
	hooksDir := filepath.Join(home, ".local", "share", "domain", "hooks")

	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		failL("settings.json ilegible (" + settingsPath + "): " + err.Error())
		return len(claudeHooks)
	}
	hooks, _ := cfg["hooks"].(map[string]any)

	fails := 0
	for _, spec := range claudeHooks {
		hookPath := filepath.Join(hooksDir, spec.Script)
		scriptOK := fileExists(hookPath)
		regOK := hooks != nil && claudeHookRegistered(hooks, spec.Event, hookPath)
		if scriptOK && regOK {
			ok(fmt.Sprintf("%s → %s (registrado + script presente)", spec.Event, spec.Script))
			continue
		}
		if !scriptOK {
			failL(fmt.Sprintf("%s → falta el script %s", spec.Event, hookPath))
		}
		if !regOK {
			failL(fmt.Sprintf("%s → %s NO registrado en settings.json", spec.Event, spec.Script))
		}
		fails++
	}
	return fails
}

// checkHookMatchers verifica que cada hook registrado en settings.json tenga el
// matcher correcto según su spec. El hook se registra con un matcher regex que
// filtra en qué eventos dispara; si el matcher no está o es incorrecto, el hook
// corre en momentos inesperados o no corre cuando debería (DOMAINSERV-76).
func checkHookMatchers(home string) int {
	step("Hook matchers (settings.json)")
	settingsPath := claudeSettingsPath(home)
	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		failL("settings.json ilegible (" + settingsPath + "): " + err.Error())
		return len(claudeHooks)
	}
	hooks, _ := cfg["hooks"].(map[string]any)

	fails := 0
	for _, spec := range claudeHooks {
		if spec.Matcher == "" {
			continue
		}
		hooksDir := filepath.Join(home, ".local", "share", "domain", "hooks")
		hookPath := filepath.Join(hooksDir, spec.Script)
		expected := spec.Matcher

		got := claudeHookGetMatcher(hooks, spec.Event, hookPath)
		if got == expected {
			ok(fmt.Sprintf("%s matcher OK: %s", spec.Script, expected))
		} else if got == "" {
			failL(fmt.Sprintf("%s (%s): sin matcher — se esperaba %q", spec.Script, spec.Event, expected))
			fails++
		} else {
			failL(fmt.Sprintf("%s (%s): matcher incorrecto %q — se esperaba %q", spec.Script, spec.Event, got, expected))
			fails++
		}
	}
	return fails
}

// claudeHookGetMatcher busca el matcher de un hook registrado en settings.json
// para un evento+command dados. Devuelve "" si no encuentra el hook o no tiene matcher.
func claudeHookGetMatcher(hooks map[string]any, event, hookPath string) string {
	arr, ok := hooks[event].([]any)
	if !ok {
		return ""
	}
	for _, entry := range arr {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hs, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == hookPath {
				matcher, _ := m["matcher"].(string)
				return matcher
			}
		}
	}
	return ""
}
