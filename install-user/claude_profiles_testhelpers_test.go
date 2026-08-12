package main

import "path/filepath"

// DOMAINSERV-279: las funciones de config de Claude pasaron de recibir el HOME a recibir el
// CONFIG DIR, porque ahora hay uno por perfil. Estos helpers mantienen la expresividad de las
// suites que razonan en términos de "el home de prueba": dicen explícitamente qué perfil de
// ese home se está mirando, en vez de dejar el .claude implícito dentro de cada helper.

// perfilDefault es el config dir default (~/.claude) bajo un home de prueba.
func perfilDefault(home string) string {
	return filepath.Join(home, ".claude")
}

// claudeSettingsPath es el settings.json del perfil default de un home de prueba.
func claudeSettingsPath(home string) string {
	return claudeSettingsPathIn(perfilDefault(home))
}

// claudeDomainMdPath y claudePersonaMdPath, ídem, para las suites de instrucciones globales.
func claudeDomainMdPath(home string) string {
	return claudeDomainMdPathIn(perfilDefault(home))
}

func claudePersonaMdPath(home string) string {
	return claudePersonaMdPathIn(perfilDefault(home))
}

func claudeGlobalPath(home string) string {
	return claudeGlobalPathIn(perfilDefault(home))
}
