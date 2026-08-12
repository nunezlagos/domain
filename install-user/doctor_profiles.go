package main

import (
	"fmt"
	"os"
)

// checkPerfilesDeClaude corre los chequeos por-perfil sobre TODOS los config dirs de Claude
// detectados, no solo sobre ~/.claude (DOMAINSERV-279).
//
// El motivo es de seguridad, no de completitud: permissions.deny y permissions.ask son POR
// PERFIL, así que un perfil sin configurar es un perfil sin el git-guard. Un doctor que solo
// mirara ~/.claude daría VERDE mientras el perfil que la sesión está usando no tiene ninguna
// barrera — el mismo falso verde de DOMAINSERV-239, donde el instalador reportaba éxito con
// los hooks viejos.
//
// Un perfil detectado pero NO configurado no suma a critical: puede ser un directorio que el
// usuario creó y todavía no instaló. Se reporta como advertencia con el comando exacto para
// arreglarlo, porque callarlo es justamente el modo de falla que este chequeo viene a cerrar.
func checkPerfilesDeClaude(home string) int {
	perfiles := detectClaudeProfiles(home, os.Getenv("CLAUDE_CONFIG_DIR"))

	critical := 0
	var sinConfigurar []string
	for _, perfil := range perfiles {
		if !perfil.Exists || !estaConfigurado(perfil) {
			sinConfigurar = append(sinConfigurar, perfil.Path)
			continue
		}
		etiqueta := perfil.Path
		if perfil.Active {
			etiqueta += " (activo)"
		}
		step("Perfil de Claude: " + etiqueta)
		critical += checkHooks(perfil.Path)
		critical += checkHookMatchers(perfil.Path)
		critical += checkPermissions(perfil.Path)
		critical += checkInstructions(perfil.Path)
		critical += checkClaudeMdExcludes(perfil.Path)
	}

	if len(sinConfigurar) > 0 {
		step("Perfiles de Claude sin configurar")
		for _, p := range sinConfigurar {
			warnL(fmt.Sprintf("%s NO tiene config de domain: sin hooks y SIN el git-guard de "+
				"permissions.deny/ask. Corré: domain-install --claude-config-dir %s", p, p))
		}
	}
	return critical
}

// estaConfigurado distingue un perfil que domain ya tocó de un directorio que existe por
// otra razón. Se mira settings.json, que es donde viven hooks y permissions: sin ese archivo
// no hay nada que los chequeos por-perfil puedan validar, y correrlos igual produciría un
// muro de fallas por un perfil que nadie instaló.
func estaConfigurado(perfil claudeProfile) bool {
	_, err := os.Stat(claudeSettingsPathIn(perfil.Path))
	return err == nil
}
