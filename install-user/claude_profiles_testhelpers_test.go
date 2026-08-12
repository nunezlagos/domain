package main

import (
	"path/filepath"
	"testing"
)

// homeAislado fija HOME a un directorio de prueba y ADEMÁS neutraliza
// CLAUDE_CONFIG_DIR.
//
// Lo segundo no es cosmética. Desde DOMAINSERV-279 el doctor resuelve el perfil con
// detectClaudeProfiles(home, os.Getenv("CLAUDE_CONFIG_DIR")), así que un test que solo
// pisa HOME sigue leyendo la variable de la máquina que lo corre: con
// CLAUDE_CONFIG_DIR=~/.claude-work exportada, el doctor audita el perfil REAL del
// desarrollador en vez del home de prueba y el test falla por el entorno. En CI la
// variable no existe, así que el fallo es invisible ahí y solo aparece en la máquina de
// quien la tiene seteada — la misma trampa que bdb95c21 ya había pagado una vez.
func homeAislado(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
}

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
