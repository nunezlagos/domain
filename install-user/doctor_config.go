package main

import (
	"fmt"
	"os/exec"
	"strings"
)

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
