package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// DOMAINSERV-239, modo de falla 2. MEDIDO el 2026-08-06: tres sitios construyen el directorio de
// hooks a mano —doctor_hooks.go:13, doctor_hooks.go:61 dentro del loop de checkHookMatchers, y
// claude_hook.go:61— mientras Paths() lo resuelve en platform.go:112 desde dataDir, que EN
// WINDOWS es %APPDATA%.
//
// El bug que eso habilita no es cosmético: si alguien alinea claude_hook.go a Paths() y se olvida
// del de doctor_hooks.go:61, en Windows el registro de settings.json apunta a
// %APPDATA%\domain\hooks y checkHookMatchers sigue mirando ~/.local/share. claudeHookGetMatcher
// devuelve "" para los tres hooks que tienen Matcher (claude_hook.go:40,42,46), o sea TRES
// FALLOS CRÍTICOS del doctor en una instalación sana.
//
// POR QUÉ UN GUARD DE FUENTE Y NO UN TEST DE COMPORTAMIENTO: el desalineo solo se manifiesta con
// p.OS == "windows", y el CI no corre en Windows. Un test que ejercite el camino real nunca lo
// atraparía en este repo. Lo único que cierra la puerta es que nadie pueda escribir la ruta a
// mano.

// hooksDirHardcodeado devuelve los sitios de código de producción que arman el directorio de
// hooks sin pasar por Paths().
func hooksDirHardcodeado(t *testing.T) []string {
	t.Helper()
	re := regexp.MustCompile(`filepath\.Join\([^)]*"\.local"[^)]*"hooks"[^)]*\)`)

	var hits []string
	err := filepath.Walk(".", func(ruta string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// platform.go es la fuente única: es el ÚNICO lugar donde la ruta se arma legítimamente
		if !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") || ruta == "platform.go" {
			return nil
		}
		b, rerr := os.ReadFile(ruta)
		if rerr != nil {
			return nil
		}
		for i, linea := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(linea), "//") {
				continue
			}
			if re.MatchString(linea) {
				hits = append(hits, ruta+":"+strconv.Itoa(i+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recorriendo el paquete: %v", err)
	}
	return hits
}

func TestHooksDir_SoloSeResuelveEnPlatform(t *testing.T) {
	hits := hooksDirHardcodeado(t)

	if len(hits) > 0 {
		t.Errorf("el directorio de hooks se construye a mano en %v.\n"+
			"Paths() lo resuelve desde dataDir, que en Windows es %%APPDATA%%: cada sitio que lo arma "+
			"con \".local/share\" queda desalineado ahí. Si el registro en settings.json y el chequeo "+
			"del doctor caen en lados distintos, claudeHookGetMatcher devuelve \"\" y el doctor "+
			"reporta 3 fallos críticos en una instalación sana. Usá paths.AgentHooksDir.", hits)
	}
}

// El guard de arriba no vale nada si Paths() dejara de resolverlo: quedaría todo alineado a nada.
func TestHooksDir_PathsLoSigueResolviendo(t *testing.T) {
	b, err := os.ReadFile("platform.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "AgentHooksDir:") {
		t.Error("Paths() dejó de resolver AgentHooksDir: el guard de fuente única no tendría a qué apuntar")
	}
}

// Y el desalineo que se quiere evitar existe porque en Windows la base cambia. Si alguien quitara
// esa bifurcación, el modo de falla desaparecería... y también el motivo de este guard: mejor que
// falle acá y se revise, a que quede un guard defendiendo algo que ya no ocurre.
func TestPaths_EnWindowsElDataDirEsAppData(t *testing.T) {
	b, err := os.ReadFile("platform.go")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?s)dataDir\s*=\s*p\.AppData\(\)`).Match(b) {
		t.Error("Paths() ya no manda dataDir a AppData en Windows: revisar si el modo de falla 2 de " +
			"DOMAINSERV-239 sigue existiendo, y si no, retirar el guard en vez de dejarlo de adorno")
	}
}
