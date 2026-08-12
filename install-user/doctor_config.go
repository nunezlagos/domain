package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// checkPermissions verifica que permissions.allow tenga mcp__domain-mcp y que
// permissions.deny tenga todas las reglas de git de domainPermissionDenies.
// Devuelve la cantidad de grupos de permisos con problemas (0, 1 o 3).
func checkPermissions(configDir string) int {
	step("Permisos (allow/deny)")
	settingsPath := claudeSettingsPathIn(configDir)
	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		failL("settings.json ilegible (" + settingsPath + "): " + err.Error())
		return 2
	}
	perms, _ := cfg["permissions"].(map[string]any)
	deny := doctorStringSet(perms["deny"])

	fails := checkRulesPresent(doctorStringSet(perms["allow"]), domainPermissionAllows,
		"permissions.allow tiene las %d reglas de domain",
		"permissions.allow le faltan reglas: %v")
	fails += checkRulesPresent(deny, domainPermissionDenies,
		"permissions.deny tiene las %d reglas de git irrecuperable",
		"permissions.deny le faltan reglas de git: %v")
	return fails + checkAskNotShadowedByDeny(perms["ask"], deny)
}

// checkRulesPresent reporta las reglas de want que faltan en have. okFmt recibe
// la cantidad esperada; failFmt, la lista de faltantes. Devuelve 1 si falta alguna.
func checkRulesPresent(have map[string]bool, want []string, okFmt, failFmt string) int {
	var missing []string
	for _, rule := range want {
		if !have[rule] {
			missing = append(missing, rule)
		}
	}
	if len(missing) > 0 {
		failL(fmt.Sprintf(failFmt, missing))
		return 1
	}
	ok(fmt.Sprintf(okFmt, len(want)))
	return 0
}

// checkAskNotShadowedByDeny verifica el ask en DOS direcciones (DOMAINSERV-278).
// Que la regla esté en ask no dice nada si además quedó en deny de un install
// previo: deny gana (orden deny → ask → allow) y el usuario sigue sin poder
// aprobar el comando. Un chequeo que solo mirara la presencia daría verde con el
// bloqueo intacto.
func checkAskNotShadowedByDeny(rawAsk any, deny map[string]bool) int {
	askSet := doctorStringSet(rawAsk)
	var missingAsk, stillDenied []string
	for _, rule := range domainPermissionAsks {
		if !askSet[rule] {
			missingAsk = append(missingAsk, rule)
		}
		if deny[rule] {
			stillDenied = append(stillDenied, rule)
		}
	}
	switch {
	case len(missingAsk) > 0:
		failL(fmt.Sprintf("permissions.ask le faltan reglas de git recuperable: %v", missingAsk))
		return 1
	case len(stillDenied) > 0:
		failL(fmt.Sprintf("estas reglas están en ask pero TAMBIÉN en deny, y deny gana: %v — "+
			"re-corré el install para que las saque de deny", stillDenied))
		return 1
	}
	ok(fmt.Sprintf("permissions.ask tiene las %d reglas de git recuperable, ninguna pisada por deny",
		len(domainPermissionAsks)))
	return 0
}

// checkInstructions verifica que existan ~/.claude/domain.md y persona.md y que
// domain.md referencie persona.md (@persona.md). Devuelve la cantidad de fallas.
func checkInstructions(configDir string) int {
	step("Instrucciones globales (domain.md + persona.md)")
	fails := 0

	domainPath := claudeDomainMdPathIn(configDir)
	personaPath := claudePersonaMdPathIn(configDir)

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
func checkClaudeMdExcludes(configDir string) int {
	step("claudeMdExcludes (settings.json)")
	settingsPath := claudeSettingsPathIn(configDir)
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
