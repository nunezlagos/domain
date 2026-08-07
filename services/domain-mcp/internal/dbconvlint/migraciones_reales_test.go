package dbconvlint

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// DOMAINSERV-249. El job db-conventions-lint del CI corre el linter sobre las migraciones
// reales, pero nada lo hacía localmente: se descubrió que estaba en rojo desde hacía días
// mirando el workflow a mano, no porque algo avisara. Los 17 tests que ya existen en este
// paquete lintean SQL inline, así que verifican las REGLAS y no el REPO — pueden estar todos
// verdes con las migraciones en rojo, que es exactamente lo que pasó.
//
// Este test cierra ese hueco: `go test ./...` deja de poder pasar con el schema violando sus
// propias convenciones.

// baselineDelCI: migraciones hasta este número quedan congeladas. Debe coincidir con el
// argumento -baseline del job db-conventions-lint; TestBaselineDelLinter_CoincideConElCI lo
// verifica, porque dos copias del mismo número divergen en silencio.
const baselineDelCI = 282

func TestLint_MigracionesDelRepo_SinViolacionesSobreElBaseline(t *testing.T) {
	dir := filepath.Join("..", "migrate", "migrations")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var violaciones []string
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		if numeroDeMigracion(e.Name()) <= baselineDelCI {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, iss := range Lint(e.Name(), string(raw)) {
			violaciones = append(violaciones, iss.File+":"+strconv.Itoa(iss.Line)+" ["+iss.Rule+"] "+iss.Message)
		}
	}

	if len(violaciones) > 0 {
		sort.Strings(violaciones)
		t.Fatalf("las migraciones posteriores a la %d violan las convenciones del proyecto (%d):\n  %s",
			baselineDelCI, len(violaciones), strings.Join(violaciones, "\n  "))
	}
}

// El baseline vive en dos lugares y ninguno importa al otro: si el CI lo sube y este test no,
// el guard local pasa a medir un rango distinto del que protege el pipeline.
func TestBaselineDelLinter_CoincideConElCI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", ".github", "workflows", "ci-mcp.yml"))
	if err != nil {
		t.Skipf("no se pudo leer el workflow: %v", err)
	}
	esperado := "-baseline " + strconv.Itoa(baselineDelCI)
	if !strings.Contains(string(raw), esperado) {
		t.Fatalf("el CI no usa %q: el guard local estaría midiendo un rango distinto al del pipeline", esperado)
	}
}

// compliance_ es un dominio funcional como webhook_ o skill_; la lista de prefijos
// simplemente no lo contemplaba cuando se escribieron las migraciones 000291 y 000292.
func TestValidTablePrefixes_IncluyeCompliance(t *testing.T) {
	if !hasValidTablePrefix("compliance_frameworks") {
		t.Fatal("compliance_ no está en la taxonomía de prefijos: las tablas del dominio no pueden nombrarse conforme")
	}
}

func numeroDeMigracion(nombre string) int {
	n, err := strconv.Atoi(strings.SplitN(nombre, "_", 2)[0])
	if err != nil {
		return 0
	}
	return n
}
