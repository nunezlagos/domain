package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// DOMAINSERV-241: SecretsStore y MCPServerService se declaraban y se asignaban en
// serverServices, y NUNCA se leían. 768 líneas de service muerto detrás de un campo
// que parecía cableado.
//
// POR QUÉ NINGÚN LINTER LO DETECTA, que es el hallazgo de la clase: los campos son
// EXPORTADOS. staticcheck marca lo no-usado solo cuando es unexported, porque para un
// campo exportado asume un consumidor fuera del paquete. Y este repo no tiene ninguna
// config de golangci-lint, así que ni eso corre. O sea que el guard tiene que ser
// propio: nadie de estante mide esto.
//
// serverServices es el grafo de inyección de dependencias del server. Un campo que
// nadie lee no es un servicio disponible: es un servicio que el server construye,
// paga (pool, cipher, logger) y no usa. El patrón ya se repitió en DOMAINSERV-201 y
// DOMAINSERV-235; este guard lo convierte en un test que falla al aparecer.

// deudaConocida son los campos que HOY se asignan y no se leen, medidos el 2026-08-05
// al escribir este guard. El valor de cada uno dice qué se midió de su paquete, porque
// "campo muerto" y "paquete muerto" son cosas distintas y el fix cambia según cuál sea.
//
// Se congelan en vez de borrarse acá por una razón concreta: borrar cada uno exige
// verificar los otros consumidores de su paquete —el análisis que se hizo para
// SecretsStore y MCPServerService— y hacerlo mal deja el server sin un servicio que
// alguien sí usaba. La baseline los deja medidos y visibles; lo que el guard impide es
// que aparezca uno NUEVO sin que nadie se entere. Mismo criterio que
// .size-lint-baseline y que el guard de writers de DOMAINSERV-244.
var deudaConocida = map[string]string{
	// paquete HUÉRFANO: solo lo importa este wiring, igual que el mcpserver que se
	// borró en DOMAINSERV-241. Candidato directo a borrado completo.
	"BillingService": "service/billing sin más importadores",
	// huérfano de hecho: el único otro importador es su propio test de integración
	"TraceService": "service/traceability solo importado por su test",
	// el paquete VIVE (internal/service/project lo usa); lo muerto es este campo
	"ProjectTemplateService": "paquete vivo, campo sin lector",
	// issuebuilder tiene 9 importadores: lo muerto es la variante Adaptive
	"IssuebuilderAdaptive": "paquete vivo, campo Adaptive sin lector",
	// skill_ab_test lo usan handlers y un cron: lo muerto es el Router
	"SkillABTestRouter": "paquete vivo, campo Router sin lector",
	// un bool de config que nadie consulta: el server lo calcula y lo tira
	"OutboundRequireTLS": "bool calculado y nunca consultado",
}

func TestServerServices_TodoCampoExportadoSeLee(t *testing.T) {
	raiz := raizDelModuloDeCmd(t)
	campos := camposDeServerServices(t, filepath.Join(raiz, "cmd", "domain", "server_services.go"))
	if len(campos) == 0 {
		t.Fatal("no se pudo parsear los campos de serverServices: el guard no está midiendo nada")
	}

	fuentes := fuentesDelModulo(t, raiz)
	var muertos []string
	for _, campo := range campos {
		// la declaración y la asignación NO cuentan como lectura: `s.X = …` es
		// escritura, y es justamente la forma que tenía el defecto
		lectura := regexp.MustCompile(`\.` + campo + `\b\s*[^=\s]|\.` + campo + `\b\s*==|\.` + campo + `\b\s*\)|\.` + campo + `\b\s*,|\.` + campo + `\b\s*$`)
		leido := false
		for _, src := range fuentes {
			for _, linea := range strings.Split(src, "\n") {
				if regexp.MustCompile(`^\s*s\.` + campo + `\s*=`).MatchString(linea) {
					continue // asignación en el propio wiring
				}
				if regexp.MustCompile(`^\s*` + campo + `\s+\*?[A-Za-z]`).MatchString(linea) {
					continue // la declaración del campo en la struct
				}
				if lectura.MatchString(linea) {
					leido = true
					break
				}
			}
			if leido {
				break
			}
		}
		if !leido {
			if _, conocido := deudaConocida[campo]; !conocido {
				muertos = append(muertos, campo)
			}
			continue
		}
		// contra-prueba: si un campo de la deuda YA se lee, la baseline quedó
		// desactualizada. Sin esto la lista crece y nunca se limpia, y termina tapando
		// un campo nuevo con una entrada vieja que ya no aplica.
		if motivo, conocido := deudaConocida[campo]; conocido {
			t.Errorf("%s figura en deudaConocida (%q) pero SÍ se lee: sacalo de la baseline", campo, motivo)
		}
	}

	sort.Strings(muertos)
	if len(muertos) > 0 {
		t.Errorf("campos NUEVOS de serverServices que se asignan y NUNCA se leen: %s\n"+
			"Un campo que nadie lee no es un servicio disponible: es uno que el server construye, "+
			"paga (pool, cipher, logger) y no usa. O se cablea a un consumidor, o se borra junto con "+
			"su paquete si tampoco tiene otro (DOMAINSERV-241). Si de verdad tiene que quedar sin "+
			"lector, agregalo a deudaConocida con lo que medís de su paquete.",
			strings.Join(muertos, ", "))
	}
}

// camposDeServerServices extrae los nombres exportados de la struct. Se parsea el
// archivo en vez de mantener una lista: una lista paralela se desincroniza y el guard
// pasaría a proteger un conjunto que ya no existe.
func camposDeServerServices(t *testing.T, ruta string) []string {
	t.Helper()
	contenido, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	texto := string(contenido)
	inicio := strings.Index(texto, "type serverServices struct {")
	if inicio == -1 {
		t.Fatal("no se encontró la struct serverServices")
	}
	fin := strings.Index(texto[inicio:], "\n}")
	if fin == -1 {
		t.Fatal("no se encontró el cierre de serverServices")
	}
	var campos []string
	for _, m := range regexp.MustCompile(`(?m)^\t([A-Z][A-Za-z0-9]*)\s+`).FindAllStringSubmatch(texto[inicio:inicio+fin], -1) {
		campos = append(campos, m[1])
	}
	return campos
}

func fuentesDelModulo(t *testing.T, raiz string) []string {
	t.Helper()
	var fuentes []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(raiz, dir), func(ruta string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(ruta, ".go") {
				return err
			}
			b, err := os.ReadFile(ruta)
			if err != nil {
				return err
			}
			fuentes = append(fuentes, string(b))
			return nil
		})
		if err != nil {
			t.Fatalf("recorrer %s: %v", dir, err)
		}
	}
	return fuentes
}

// raizDelModuloDeCmd sube hasta el go.mod en vez de contar `..` desde el cwd.
//
// Contar niveles asume que el runner posiciona el cwd en el directorio del paquete, y
// eso NO se cumple siempre: con `go test ./cmd/domain/` el test pasaba y con
// `go test ./...` el mismo test resolvía la raíz del REPO en vez del módulo y fallaba.
// Un guard que depende de cómo se lo invoque falla por el motivo equivocado.
func raizDelModuloDeCmd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			t.Fatalf("no se encontró go.mod subiendo desde %s", dir)
		}
		dir = padre
	}
}
