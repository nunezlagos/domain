package main

import (
	"bytes"
	"os"
	"path/filepath"
)

// resultadoHook dice qué pasó con un hook al intentar instalarlo. Se devuelve en vez de solo un
// error porque "no hice nada porque ya estaba bien" y "no hice nada porque el disco difiere" son
// desenlaces distintos: el segundo hay que reportarlo.
type resultadoHook int

const (
	hookAlDia      resultadoHook = iota // el disco ya tiene exactamente lo embebido
	hookEscrito                         // no existía y se instaló
	hookDivergente                      // existe y difiere: NO se pisa
	hookFallo                           // no se pudo leer el embebido o escribir el destino
)

func (r resultadoHook) String() string {
	switch r {
	case hookAlDia:
		return "al-dia"
	case hookEscrito:
		return "escrito"
	case hookDivergente:
		return "divergente"
	default:
		return "fallo"
	}
}

// instalarHookEmbebido escribe un lifecycle hook desde el binario, SIN PISAR uno que difiera
// (DOMAINSERV-239, modo de falla 3).
//
// La política es deliberada y se decidió con el usuario. El archivo en juego puede ser
// domain-pre-edit.sh, o sea el gate SDD: un binario de un tag ANTERIOR corriendo sobre una
// máquina al día lo degradaría a su versión vieja, y escribirHookEmbebido no tiene forma de
// saber cuál de los dos es más nuevo. Ante la duda no se toca y se avisa — un downgrade
// silencioso del gate es peor que un upgrade que no ocurre, porque el segundo se ve.
//
// Consecuencia asumida: un upgrade legítimo con hooks nuevos TAMPOCO pisa. Para actualizarlos hay
// que borrar el viejo, y el doctor reporta la divergencia hasta que se haga.
func instalarHookEmbebido(script, hooksDir string) resultadoHook {
	embebido, err := hooksFS.ReadFile("hooks/" + script)
	if err != nil {
		return hookFallo
	}
	destino := filepath.Join(hooksDir, script)

	if actual, rerr := os.ReadFile(destino); rerr == nil {
		if bytes.Equal(actual, embebido) {
			return hookAlDia
		}
		return hookDivergente
	}

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return hookFallo
	}
	// 0o755 y no 0o644: un hook sin permiso de ejecución no corre, y ese fallo es mudo
	if err := os.WriteFile(destino, embebido, 0o755); err != nil {
		return hookFallo
	}
	return hookEscrito
}

// divergeDelEmbebido dice si el hook en disco NO es el que trae el binario.
//
// Ante la duda devuelve false: si no se puede leer el embebido o el archivo, no hay evidencia de
// divergencia y afirmarla sería ruido. La ausencia del archivo ya la reporta fileExists.
func divergeDelEmbebido(script, hookPath string) bool {
	embebido, err := hooksFS.ReadFile("hooks/" + script)
	if err != nil {
		return false
	}
	actual, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}
	return !bytes.Equal(actual, embebido)
}

// instalarHooksLifecycle instala los hooks que claudeHooks declara y reporta el desenlace de cada
// uno. Devuelve la cantidad de divergentes, para que el caller decida si eso merece ruido.
//
// DOMAINSERV-239, modo de falla 1: esto corre en la zona de installGlobalAssets y NO al final del
// install. runInstall tiene os.Exit(1) más abajo, y si los hooks se instalaran después de esos
// puntos, un fallo en cualquiera de ellos dejaría la máquina SIN hooks en un install limpio —
// donde hoy quedaban correctos porque los escribía el script antes de invocar al binario.
func instalarHooksLifecycle(hooksDir string) int {
	// domain-hooks-lib.sh NO está en claudeHooks porque no es un hook registrado: es la lib que
	// TODOS cargan con `. "$LIB"`. Iterar solo claudeHooks la dejaría afuera y los hooks fallarían
	// en su primera línea útil, con un error mudo que se ve como "el gate no anda".
	archivos := []string{"domain-hooks-lib.sh"}
	for _, spec := range claudeHooks {
		archivos = append(archivos, spec.Script)
	}

	divergentes := 0
	for _, script := range archivos {
		switch instalarHookEmbebido(script, hooksDir) {
		case hookEscrito:
			ok("hook instalado: " + script)
		case hookDivergente:
			divergentes++
			warnL("hook NO actualizado porque el disco difiere del binario: " + script +
				" — si tu instalación es más nueva esto es lo esperado; si no, borralo y reinstalá")
		case hookFallo:
			warnL("no se pudo instalar el hook: " + script)
		}
	}
	return divergentes
}
