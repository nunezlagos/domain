package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// runDoctor valida que la instalación del cliente domain esté completa y
// consistente bajo HOME, y reporta cada chequeo con ✓ (ok) o ✗ (falla).
// Chequea, en orden:
//   - los hooks de claudeHooks están registrados en ~/.claude/settings.json Y
//     sus scripts existen en ~/.local/share/domain/hooks/;
//   - permissions.allow tiene mcp__domain-mcp y permissions.deny tiene las
//     reglas de git de domainPermissionDenies;
//   - ~/.claude/domain.md y ~/.claude/persona.md existen y persona.md está
//     referenciada desde domain.md (@persona.md);
//   - el MCP domain responde (salud del VPS, best-effort).
//
// Devuelve el exit code: 0 si TODO lo crítico pasó, !=0 si falta algo crítico.
// La salud del MCP NO es crítica: el VPS puede estar caído sin que la
// instalación local del cliente esté rota (degradación graciosa).
func runDoctor(home string) int {
	step("domain doctor — self-check")

	critical := 0
	critical += checkHooks(home)
	critical += checkPermissions(home)
	critical += checkInstructions(home)
	checkMCPHealth(home) // best-effort, no suma a critical

	step("Resumen")
	if critical == 0 {
		ok("instalación consistente — todos los chequeos críticos pasaron")
		return 0
	}
	failL(fmt.Sprintf("%d chequeo(s) crítico(s) fallaron — re-corré el install para reparar", critical))
	return 1
}

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

// checkPermissions verifica que permissions.allow tenga mcp__domain-mcp y que
// permissions.deny tenga todas las reglas de git de domainPermissionDenies.
// Devuelve la cantidad de grupos de permisos con problemas (0, 1 o 2).
func checkPermissions(home string) int {
	step("Permisos (allow/deny)")
	settingsPath := claudeSettingsPath(home)
	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		failL("settings.json ilegible (" + settingsPath + "): " + err.Error())
		return 2
	}
	perms, _ := cfg["permissions"].(map[string]any)

	fails := 0

	allow := doctorStringSet(perms["allow"])
	if allow["mcp__domain-mcp"] {
		ok("permissions.allow tiene mcp__domain-mcp")
	} else {
		failL("permissions.allow NO tiene mcp__domain-mcp")
		fails++
	}

	deny := doctorStringSet(perms["deny"])
	var missing []string
	for _, rule := range domainPermissionDenies {
		if !deny[rule] {
			missing = append(missing, rule)
		}
	}
	if len(missing) == 0 {
		ok(fmt.Sprintf("permissions.deny tiene las %d reglas de git", len(domainPermissionDenies)))
	} else {
		failL(fmt.Sprintf("permissions.deny le faltan reglas de git: %v", missing))
		fails++
	}
	return fails
}

// checkInstructions verifica que existan ~/.claude/domain.md y persona.md y que
// domain.md referencie persona.md (@persona.md). Devuelve la cantidad de fallas.
func checkInstructions(home string) int {
	step("Instrucciones globales (domain.md + persona.md)")
	fails := 0

	domainPath := claudeDomainMdPath(home)
	personaPath := claudePersonaMdPath(home)

	domainBody, domainExists, err := readIfExists(domainPath)
	if err != nil {
		failL("no pude leer " + domainPath + ": " + err.Error())
		fails++
	} else if domainExists {
		ok("~/.claude/domain.md presente")
	} else {
		failL("falta ~/.claude/domain.md")
		fails++
	}

	if _, personaExists, err := readIfExists(personaPath); err != nil {
		failL("no pude leer " + personaPath + ": " + err.Error())
		fails++
	} else if personaExists {
		ok("~/.claude/persona.md presente")
	} else {
		failL("falta ~/.claude/persona.md")
		fails++
	}

	// persona.md debe estar referenciada desde domain.md (@persona.md).
	if domainExists {
		if strings.Contains(domainBody, "persona.md") {
			ok("persona.md referenciada desde domain.md")
		} else {
			failL("domain.md no referencia persona.md (@persona.md)")
			fails++
		}
	}
	return fails
}

// checkMCPHealth chequea, best-effort, que el VPS del MCP domain responda.
// NO es crítico: si no hay VPS_URL o el VPS no responde, avisa y sigue (la
// instalación local puede estar intacta con el VPS temporalmente caído).
func checkMCPHealth(home string) {
	step("Salud del MCP domain")
	envPath := filepath.Join(home, ".config", "domain", "install.env")
	env, err := loadEnv(envPath)
	if err != nil || env.VPSURL == "" {
		warnL("sin VPS_URL en install.env — chequeo omitido (no crítico)")
		return
	}
	url := strings.TrimRight(env.VPSURL, "/")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := pingVPS(ctx, url); err != nil {
		warnL("VPS no responde en " + url + ": " + err.Error() + " (no crítico: degrada gracioso)")
		return
	}
	ok("VPS responde en " + url)
}

// doctorStringSet convierte un array JSON de strings en un set para lookup.
// Tolera tipos mixtos (ignora los no-string) y valores ausentes.
func doctorStringSet(v any) map[string]bool {
	set := map[string]bool{}
	arr, ok := v.([]any)
	if !ok {
		return set
	}
	for _, e := range arr {
		if s, ok := e.(string); ok {
			set[s] = true
		}
	}
	return set
}
