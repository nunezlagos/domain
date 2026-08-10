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
	hooksDir := HooksDirDelSistema()

	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		failL("settings.json ilegible (" + settingsPath + "): " + err.Error())
		return len(claudeHooks)
	}
	hooks, _ := cfg["hooks"].(map[string]any)
	manifiesto := cargarManifiesto(DetectPlatform().Paths().HooksManifest)

	fails := 0
	// DOMAINSERV-267: domain-hooks-lib.sh se audita aunque NO esté en claudeHooks. No es un hook
	// registrado —es la lib que todos cargan con `. "$LIB"`— así que iterar solo claudeHooks la
	// dejaba afuera: quedaba vieja sin que nada lo dijera, y los hooks fallaban en su primera
	// línea útil con un error que se lee como "el gate no anda".
	if libPath := filepath.Join(hooksDir, "domain-hooks-lib.sh"); fileExists(libPath) {
		reportarProcedencia("(lib)", "domain-hooks-lib.sh", libPath, manifiesto)
	} else {
		failL("falta domain-hooks-lib.sh: los hooks que la cargan fallan en su primera línea útil")
		fails++
	}

	for _, spec := range claudeHooks {
		hookPath := filepath.Join(hooksDir, spec.Script)
		scriptOK := fileExists(hookPath)
		regOK := hooks != nil && claudeHookRegistered(hooks, spec.Event, hookPath)
		// DOMAINSERV-239: hasta acá el chequeo era solo fileExists, porque los hooks no estaban
		// embebidos y no había con qué comparar. Ahora sí: un hook presente pero DIVERGENTE del
		// binario se reporta, que es la diferencia entre "el archivo está" y "el archivo es el
		// que debería ser".
		if scriptOK && regOK {
			reportarProcedencia(spec.Event, spec.Script, hookPath, manifiesto)
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

// reportarProcedencia dice de QUIÉN es la diferencia cuando el disco no coincide con el
// binario. Antes las dos causas salían por el mismo warning —"el contenido DIFIERE"— y no se
// podía saber cuál era: la que se arregla corriendo el instalador, o la que se perdería si
// alguien lo corre (DOMAINSERV-267).
//
// Ninguno de los dos casos suma a `fails`, y es deliberado. "Quedó viejo" lo resuelve el
// propio instalador en su próxima corrida, así que no es un estado que exija intervención; y
// "lo editaste vos" es una decisión del usuario, no un defecto. Marcarlos críticos dejaría el
// doctor en rojo permanente y la presión operativa terminaría sacando el guard —el modo de
// falla que el ticket advierte— sin que ninguno de los dos casos lo amerite.
func reportarProcedencia(evento, script, hookPath string, manifiesto manifiestoAgentes) {
	embebido, err := hooksFS.ReadFile("hooks/" + script)
	if err != nil {
		warnL(fmt.Sprintf("%s → %s: no está embebido en este binario, no hay contra qué compararlo", evento, script))
		return
	}
	switch clasificar(hookPath, embebido, manifiesto[script]) {
	case alDia:
		ok(fmt.Sprintf("%s → %s (registrado + hash coincide)", evento, script))
	case deDomain:
		warnL(fmt.Sprintf("%s → %s: quedó de una versión anterior — corré el instalador y se actualiza solo",
			evento, script))
	default:
		warnL(fmt.Sprintf("%s → %s: tiene cambios locales, el instalador NO lo va a pisar",
			evento, script))
	}
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
		hooksDir := HooksDirDelSistema()
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
