package main

import (
	"context"
	"fmt"
	"os/exec"
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
	critical += checkPython3()
	critical += checkHooks(home)
	critical += checkHookMatchers(home)
	critical += checkMCPEntry(home)
	critical += checkPermissions(home)
	critical += checkInstructions(home)
	critical += checkClaudeMdExcludes(home)
	critical += checkOpencodePermission(home)
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

// checkMCPEntry verifica que el MCP entry exista en los archivos de configuración
// de los clientes detectados (Claude Code y OpenCode) con url + header Authorization.
// Sin entry MCP, las tools domain_* no están disponibles (DOMAINSERV-76).
func checkMCPEntry(home string) int {
	step("Entry MCP (clients.json)")
	fails := 0

	for _, path := range []string{
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".config", "opencode", "opencode.json"),
	} {
		if !fileExists(path) {
			continue
		}
		cfg, err := loadOrEmptyJSON(path)
		if err != nil {
			failL(path + " ilegible: " + err.Error())
			fails++
			continue
		}
		// Claude Code: top-level "mcpServers"; OpenCode: top-level "mcp"
		var servers map[string]any
		if s, _ := cfg["mcpServers"].(map[string]any); s != nil {
			servers = s
		} else if s, _ := cfg["mcp"].(map[string]any); s != nil {
			servers = s
		}
		if servers == nil {
			failL(path + ": sin mcpServers/mcp")
			fails++
			continue
		}
		entry, found := servers["domain-mcp"].(map[string]any)
		if !found || entry == nil {
			failL(path + ": falta entry 'domain-mcp' en mcpServers/mcp")
			fails++
			continue
		}
		urlVal, _ := entry["url"].(string)
		headers, _ := entry["headers"].(map[string]any)
		authVal, _ := headers["Authorization"].(string)
		if urlVal == "" {
			failL(path + ": entry domain-mcp sin url")
			fails++
		}
		if authVal == "" {
			failL(path + ": entry domain-mcp sin header Authorization (Bearer)")
			fails++
		}
		if urlVal != "" && authVal != "" {
			ok(path + ": entry domain-mcp presente (url + Authorization)")
		}
	}
	return fails
}

// checkClaudeMdExcludes verifica que claudeMdExcludes en settings.json tenga los
// globs de instrucciones locales neutralizadas. Sin esto, AGENTS.md/CLAUDE.md de
// proyecto pueden colisionar con las reglas globales de domain (DOMAINSERV-76).
func checkClaudeMdExcludes(home string) int {
	step("claudeMdExcludes (settings.json)")
	settingsPath := claudeSettingsPath(home)
	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		failL("settings.json ilegible (" + settingsPath + "): " + err.Error())
		return 1
	}
	excludes, _ := cfg["claudeMdExcludes"].([]any)
	have := doctorStringSet(excludes)

	var missing []string
	for _, g := range localInstructionGlobs {
		if !have[g] {
			missing = append(missing, g)
		}
	}
	if len(missing) == 0 {
		ok("todos los globs de instrucciones locales neutralizados")
		return 0
	}
	failL(fmt.Sprintf("faltan %d glob(s) en claudeMdExcludes: %v", len(missing), missing))
	return 1
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

// checkOpencodePermission verifica que opencode.json tenga el bloque
// `permission` con las reglas deny de git destructivo (DOMAINSERV-69).
func checkOpencodePermission(home string) int {
	step("Permisos OpenCode (opencode.json)")
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if !fileExists(path) {
		info("opencode no detectado — chequeo omitido")
		return 0
	}
	m, err := loadOrEmptyJSON(path)
	if err != nil {
		failL(path + " ilegible: " + err.Error())
		return 1
	}
	perm, _ := m["permission"].(map[string]any)
	if perm == nil {
		failL(path + ": falta bloque 'permission'")
		return 1
	}
	bashRules, _ := perm["bash"].(map[string]any)
	if bashRules == nil {
		failL(path + ": permission falta 'bash'")
		return 1
	}
	var missing []string
	for _, rule := range opencodeGitDenyRules {
		if v, ok := bashRules[rule]; !ok || fmt.Sprint(v) != "deny" {
			missing = append(missing, rule)
		}
	}
	if len(missing) > 0 {
		failL(fmt.Sprintf("faltan reglas deny en permission.bash: %v", missing))
		return 1
	}
	ok("todas las reglas git deny presentes en permission.bash")
	return 0
}

// checkPython3 verifica que python3 esté disponible en el PATH. Es requerido
// por los hooks del gate SDD (pre-edit/post-test/post-orchestrate). Sin python3
// el gate falla abierto (DOMAINSERV-71).
func checkPython3() int {
	step("Dependencias del sistema")
	if _, err := exec.LookPath("python3"); err != nil {
		failL("python3 no está en el PATH — requerido por el gate SDD (hooks pre-edit/post-test/post-orchestrate). Instala python3 y re-corre el doctor.")
		return 1
	}
	ok("python3 en PATH")
	return 0
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
