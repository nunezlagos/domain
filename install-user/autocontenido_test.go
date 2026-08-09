package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// DOMAINSERV-272: install-user es su propio módulo y el CI lo trata como autocontenido —
// hasta ahora el path filter decía solo install-user/**. Dos invariantes distintos, y
// conviene no confundirlos:
//
//  1. El módulo NO tiene dependencias externas (stdlib-only). Eso es lo que hace que el
//     binario del instalador se pueda compilar en cualquier runner sin go.sum ni red, y es
//     un invariante REAL que hoy nada sostiene.
//  2. El módulo SÍ lee archivos de fuera de su directorio, y eso NO es un defecto a
//     prohibir: version_test.go asserta sobre los workflows de CI a propósito. Lo que estaba
//     mal era el path filter, que no los incluía. Se arregló en el mismo commit.
//
// El tercer bullet del ticket —"ningún go:embed apunta afuera del módulo"— no se testea:
// es imposible por construcción. Medido en un módulo de prueba: `//go:embed ../afuera.txt`
// da "invalid pattern syntax" en tiempo de compilación, y este workflow ya compila con
// `go vet` y `go test`. Un test ahí sería uno que no puede fallar.

func TestAutocontenido_ElModuloNoTieneDependenciasExternas(t *testing.T) {
	raw, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	texto := string(raw)

	// `require` cubre las dos formas: el bloque y la línea suelta
	if strings.Contains(texto, "require") {
		t.Fatalf("install-user/go.mod declara dependencias externas:\n%s\n\n"+
			"El módulo es stdlib-only a propósito: así el instalador compila en cualquier "+
			"runner sin go.sum ni acceso a red, que es lo que hace viable publicarlo como "+
			"Release cross-platform. Si la dependencia es necesaria de verdad, hay que "+
			"decidirlo explícitamente y actualizar este test con la razón.", texto)
	}
	if _, err := os.Stat("go.sum"); err == nil {
		t.Fatal("apareció install-user/go.sum: el módulo dejó de ser stdlib-only")
	}
}

// El go.mod es la declaración; esto mide lo que el toolchain resuelve de verdad. Un import
// externo sin require no compila, pero este test da el mensaje correcto en vez de un error
// de build críptico, y cubre el caso de que alguien agregue un vendor/.
func TestAutocontenido_TodosLosImportsSonStdlib(t *testing.T) {
	salida, err := exec.Command("go", "list", "-f", "{{join .Imports \"\\n\"}}{{\"\\n\"}}{{join .TestImports \"\\n\"}}", "./...").Output()
	if err != nil {
		t.Skipf("no se pudo ejecutar go list: %v", err)
	}

	var externos []string
	for _, imp := range strings.Split(string(salida), "\n") {
		imp = strings.TrimSpace(imp)
		if imp == "" {
			continue
		}
		// un import de stdlib nunca tiene punto en su primer segmento (dominio)
		primero := imp
		if i := strings.Index(imp, "/"); i >= 0 {
			primero = imp[:i]
		}
		if strings.Contains(primero, ".") {
			externos = append(externos, imp)
		}
	}
	if len(externos) > 0 {
		sort.Strings(externos)
		t.Fatalf("hay imports fuera de la stdlib:\n  %s", strings.Join(externos, "\n  "))
	}
}

// El guard que cierra el bug REAL del ticket: si un test lee un archivo de fuera del módulo,
// el path filter del workflow tiene que incluir ese directorio. Si no, el test existe y no
// lo dispara nadie cuando cambia justo el archivo que asserta — que es lo que venía pasando
// con los 4 workflows de CI (15 de 18 commits no dispararon este job).
func TestAutocontenido_LoQueSeLeeDeAfueraEstaEnElPathFilter(t *testing.T) {
	// captura los segmentos literales del Join para poder comparar contra el patrón del
	// filtro, que puede ser más específico que el primer segmento: `.github/workflows/**`
	// cubre `Join("..", ".github", "workflows", x)` aunque el primer segmento sea `.github`.
	reSubida := regexp.MustCompile(`filepath\.Join\(\s*"\.\."\s*((?:,\s*"[^"]+"\s*)+)`)

	matches, err := filepath.Glob("*_test.go")
	if err != nil || len(matches) == 0 {
		t.Fatalf("no se encontró ningún _test.go: el guard quedaría verde por vacío (err=%v)", err)
	}

	dirs := map[string][]string{}
	for _, f := range matches {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		reSeg := regexp.MustCompile(`"([^"]+)"`)
		for _, m := range reSubida.FindAllStringSubmatch(string(b), -1) {
			var segs []string
			for _, s := range reSeg.FindAllStringSubmatch(m[1], -1) {
				segs = append(segs, s[1])
			}
			if len(segs) == 0 {
				continue
			}
			// el último segmento suele ser el archivo (o una variable): el filtro se
			// expresa por directorio, así que se compara contra el path sin él
			ruta := strings.Join(segs, "/")
			dirs[ruta] = append(dirs[ruta], f)
		}
	}
	if len(dirs) == 0 {
		return // nada sale del módulo: nada que exigirle al filtro
	}

	wf, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci-install-user.yml"))
	if err != nil {
		t.Fatal(err)
	}
	filtro := string(wf)

	// cubierto = algún patrón del filtro es prefijo del path leído. `.github/workflows/**`
	// cubre `.github/workflows/ci-mcp.yml`, y `.github/**` también.
	cubierto := func(ruta string) bool {
		partes := strings.Split(ruta, "/")
		for i := len(partes); i > 0; i-- {
			if strings.Contains(filtro, "'"+strings.Join(partes[:i], "/")+"/**'") {
				return true
			}
		}
		return false
	}

	var faltan []string
	for d, quien := range dirs {
		if !cubierto(d) {
			faltan = append(faltan, d+" (leído en "+strings.Join(quien, ", ")+")")
		}
	}
	if len(faltan) > 0 {
		sort.Strings(faltan)
		t.Fatalf("hay tests que leen archivos de fuera del módulo y el path filter de "+
			"ci-install-user.yml no incluye su directorio:\n  %s\n\n"+
			"Sin eso, cambiar ese archivo NO dispara este workflow: el test que lo asserta "+
			"existe y no lo ejecuta nadie hasta el próximo cambio de install-user/. Medido: "+
			"pasó en 15 de 18 commits.", strings.Join(faltan, "\n  "))
	}
}
