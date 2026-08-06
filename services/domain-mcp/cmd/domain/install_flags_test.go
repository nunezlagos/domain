package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-234: existía un test llamado TestSabotage_ParseInstallFlags_UnknownFlag en
// internal/cli/install/sabotage_test.go cuyo cuerpo era ENTERO un
// `t.Skip("integration test, requires built binary; verified manually")`. Dos problemas, y el
// segundo es el que importa:
//
//  1. El skip era incondicional y el archivo no tenía build tag, así que el test estaba muerto
//     en el job unitario Y en el de integración. No podía fallar nunca.
//  2. La razón declarada era FALSA. parseInstallFlags no necesita un binario construido: es una
//     función pura que recorre un []string. Lo que pasaba es que el test estaba en el PAQUETE
//     EQUIVOCADO —internal/cli/install— y la función vive en cmd/domain, sin exportar, así que
//     desde ahí era inalcanzable. El skip disfrazaba un test que nunca se pudo escribir.
//
// "verified manually" es exactamente lo que la policy guards-deben-ejecutarse prohíbe: si el
// paso depende de que una persona lo recuerde, no está hecho. Este archivo lo reemplaza por la
// prueba real, en el paquete donde la función existe.

func TestParseInstallFlags_FlagDesconocido_DevuelveErrorQueLoNombra(t *testing.T) {
	_, err := parseInstallFlags([]string{"--mode", "local", "--flag-que-no-existe"})

	require.Error(t, err, "un flag desconocido tiene que ser un error y no ignorarse en silencio")
	require.Contains(t, err.Error(), "--flag-que-no-existe",
		"el error tiene que NOMBRAR el flag: un 'unknown flag' pelado obliga a adivinar cuál de "+
			"todos los argumentos estaba mal escrito")
}

func TestParseInstallFlags_FlagConValorFaltante_NoConsumeElSiguiente(t *testing.T) {
	// --mode al final, sin valor: el error tiene que ser explícito y no un índice fuera de rango
	_, err := parseInstallFlags([]string{"--base-url", "http://x", "--mode"})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "--mode",
		"el error de valor faltante tiene que decir a QUÉ flag le falta")
}

func TestParseInstallFlags_Help_DevuelveErrHelpYNoUnErrorComun(t *testing.T) {
	_, err := parseInstallFlags([]string{"--help"})
	require.ErrorIs(t, err, errHelp,
		"--help se distingue de un error real con errHelp: quien llama imprime la ayuda y sale "+
			"con 0, no con 1")
}

func TestParseInstallFlags_SinArgs_TraeLosDefaultsDocumentados(t *testing.T) {
	f, err := parseInstallFlags(nil)
	require.NoError(t, err)

	// el default de agents es AMBOS asistentes, y está documentado en el comentario de la
	// función: si alguien lo cambia a uno solo, un install deja de registrar el otro sin aviso
	require.ElementsMatch(t, []string{"opencode", "claude-code"}, f.agents,
		"el default registra el MCP en los dos asistentes")
	require.NotEmpty(t, f.baseURL, "baseURL tiene default (env o localhost), nunca queda vacío")
	require.NotNil(t, f.features, "features nil haría panic en el primer --enable-X")
}

func TestParseInstallFlags_NoClaudeCode_QuitaSoloEse(t *testing.T) {
	f, err := parseInstallFlags([]string{"--no-claude-code"})
	require.NoError(t, err)
	require.Equal(t, []string{"opencode"}, f.agents,
		"--no-claude-code quita uno puntualmente y deja el otro: si vacía la lista, el install "+
			"no registra nada y no lo dice")
}

// El error de flag desconocido no puede ser errHelp: si lo fuera, un typo saldría con código 0
// y el usuario creería que el install corrió.
func TestParseInstallFlags_FlagDesconocido_NoSeConfundeConHelp(t *testing.T) {
	_, err := parseInstallFlags([]string{"--typo"})
	require.Error(t, err)
	require.False(t, errors.Is(err, errHelp),
		"un typo tiene que salir con error real, no con el camino de --help que sale con 0")
}
