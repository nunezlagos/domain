package flow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-218, criterio 3. ValidarParticionDisjunta vivió desde el incremento 1 con 6 tests
// verdes y CERO callers en producción: el criterio parecía cumplido mirando el paquete flow y no
// lo estaba mirando el handler. Este test es el guard sobre el guard — si alguien vuelve a
// dejarla huérfana, falla acá y no dentro de seis meses en una auditoría.
//
// Se busca en el ÁRBOL DE FUENTES y no con reflexión porque lo que se quiere afirmar no es que
// la función exista, sino que haya un camino de ejecución que la alcance.

func callersEnProduccion(t *testing.T, simbolo string, raices ...string) []string {
	t.Helper()
	var hits []string
	for _, raiz := range raices {
		err := filepath.Walk(raiz, func(ruta string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(ruta)
			if err != nil {
				return nil
			}
			for _, linea := range strings.Split(string(b), "\n") {
				// se descartan comentarios: una mención en prosa no es un camino de ejecución
				podada := strings.TrimSpace(linea)
				if strings.HasPrefix(podada, "//") || !strings.Contains(podada, simbolo+"(") {
					continue
				}
				// la definición no cuenta como caller
				if strings.HasPrefix(podada, "func "+simbolo) {
					continue
				}
				hits = append(hits, ruta)
				break
			}
			return nil
		})
		if err != nil {
			t.Fatalf("recorriendo %s: %v", raiz, err)
		}
	}
	return hits
}

func TestValidarParticionDisjunta_TieneCallerEnProduccion(t *testing.T) {
	// SolapamientoConOtros es su caller, y a su vez tiene que tener el suyo: una cadena que
	// termina en una función que nadie invoca sigue siendo un guard inerte.
	for _, simbolo := range []string{"ValidarParticionDisjunta", "primerSolape", "SolapamientoConOtros"} {
		hits := callersEnProduccion(t, simbolo, "..", "../../mcp")
		if len(hits) == 0 {
			t.Errorf("%s no tiene NINGÚN caller en código de producción: es un guard inerte y viola "+
				"la policy guards-deben-ejecutarse. Un guard que no se ejecuta no es un guard.", simbolo)
		}
	}
}
