//go:build integration

// Guard de DOMAINSERV-235: PREPARA también el SQL escrito A MANO, el que no pasa por sqlc.
//
// POR QUÉ EXISTE, y es la razón estructural de un bug real: el guard hermano de
// DOMAINSERV-199 (prepare_integration_test.go) recolecta las queries de los `query.sql.go`
// GENERADOS por sqlc. Su baseline declaraba "415/415 preparan" y estaba en verde, pero
// `internal/admin/runners_usage` usaba `pool.Query` crudo y filtraba
// `agent_runs WHERE trigger_type = ...` — una columna que NO EXISTE en esa tabla, medido
// contra prod: las columnas de agent_runs son id, agent_id, flow_run_id, parent_run_id,
// user_id, status, inputs, outputs, error, cancellation_reason, tokens_input, tokens_output,
// cost_usd, iterations, started_at, finished_at, created_at, updated_at, metadata, source,
// project_id y model. `trigger_type` solo vive en flow_runs.
//
// Ese paquete se borró por muerto, pero el HUECO del guard sobrevive al borrado: cualquier
// query cruda nueva con una columna fantasma vuelve a pasar sin que nadie la prepare. Un
// baseline que declara 100% sobre el subconjunto que sabe leer no es cobertura, es una cifra
// tranquilizadora.
//
// LO QUE ESTE GUARD NO PUEDE VER, declarado para que nadie lea su verde como "todo el SQL
// crudo está sano":
//  1. El SQL armado por CONCATENACIÓN o con verbos de fmt. Su texto final no existe en el
//     fuente, así que PREPARE no tiene qué recibir. Se cuentan y se listan, no se silencian.
//  2. Un caso REAL que cae justo ahí y queda sin cubrir: `internal/mcp/server/memory_tools.go`
//     (~línea 342) tiene un GROUP BY al que le falta `observation_type` — se midió con la
//     primera versión de este guard, que agarró el fragmento suelto y PostgreSQL lo rechazó
//     con 42803. Al excluir los fragmentos —correcto, porque preparar un pedazo no dice nada—
//     esa query salió del alcance. El bug NO está arreglado y NO está cubierto: cubrirlo exige
//     evaluar la concatenación completa, que es un guard distinto.
//  3. Los objetos de EXTENSIONES de PostgreSQL (pg_stat_statements): existen en prod si la
//     extensión está instalada y nunca en el container de test.
package dbguard_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type queryCruda struct {
	archivo string
	linea   int
	sql     string
}

// hallazgosCongelados son las queries crudas que HOY no preparan, con su causa medida. El
// guard falla ante una NUEVA, no ante estas: es el patrón de .size-lint-baseline y del
// baseline de DOMAINSERV-244 —la deuda vieja no bloquea, lo nuevo sí— establecido en
// DOMAINSERV-198 sobre 88 violaciones que no eran corregibles retroactivamente.
//
// Cada entrada dice qué está roto. Sacar una de acá exige arreglar la query, y agregar una
// exige explicar por qué se acepta: un baseline sin razones se vuelve un cajón donde todo
// entra.
var hallazgosCongelados = map[string]string{
	// la tabla organizations la dropeó la 000143 (modelo single-tenant, DOMAINSERV-187).
	// Son comandos de dev/seed, no corren en producción, pero están rotos desde entonces.
	"cmd/domain/dev_bootstrap.go:96": "relation organizations does not exist — dropeada por la 000143",
	"cmd/domain/seed_demo.go:96":     "relation organizations does not exist — dropeada por la 000143",
	// tablas que ninguna migración crea: el paquete quedó escrito contra un schema que no llegó
	"internal/context/semcache/cache.go:108": "relation llm_semantic_cache does not exist — ninguna migración la crea",
	"internal/context/semcache/cache.go:169": "relation llm_semantic_cache does not exist — ninguna migración la crea",
	"internal/service/cron/events.go:120":    "relation event_log does not exist — ninguna migración la crea",
	"internal/service/cron/events.go:135":    "relation event_log does not exist — ninguna migración la crea",
	// bug de SQL de verdad, y rompe en cuanto alguien ejecute esa ruta
	"internal/service/skill/auto_engine.go:81": "input_schema es jsonb y recibe text sin cast (42804)",
	// pg_stat_statements es una EXTENSIÓN, no una tabla del schema: existe en prod si está
	// instalada y nunca en el container de testcontainers. No es un defecto del código.
	"internal/dbstats/service.go:114": "pg_stat_statements es una extensión, ausente en el container de test",
}

func TestQueriesCrudas_TodasPreparanContraElSchemaReal(t *testing.T) {
	crudas, noAnalizables := recolectarCrudas(t)
	require.NotEmpty(t, crudas,
		"el recolector no encontró NINGUNA query cruda: o el repo dejó de tener SQL a mano "+
			"—improbable— o el predicado dejó de reconocerlo, y entonces este guard no mide nada")

	conn := conectarConMigracionesAplicadas(t)
	ctx := context.Background()

	var fallos, congeladosVistos []string
	for i, q := range crudas {
		_, err := conn.Prepare(ctx, fmt.Sprintf("guard_cruda_%d", i), q.sql)
		if err == nil {
			continue
		}
		clave := fmt.Sprintf("%s:%d", q.archivo, q.linea)
		// 42P18 = "could not determine data type of parameter": PREPARE pide inferir el tipo de
		// $1 sin el contexto que pgx sí tiene al ejecutar con argumentos tipados. Es una
		// limitación del MÉTODO y no un defecto de la query, así que no cuenta como fallo — pero
		// tampoco se afirma que esté cubierta.
		if strings.Contains(err.Error(), "42P18") {
			noAnalizables = append(noAnalizables, q)
			continue
		}
		if _, congelado := hallazgosCongelados[clave]; congelado {
			congeladosVistos = append(congeladosVistos, clave)
			continue
		}
		fallos = append(fallos, fmt.Sprintf("%s: %v", clave, err))
	}

	require.Empty(t, fallos,
		"hay SQL escrito a mano NUEVO que PostgreSQL rechaza al preparar. Son bugs latentes: el "+
			"compilador no mira dentro de un string y sqlc no ve estas queries. Si de verdad no "+
			"se puede arreglar ahora, agregalo a hallazgosCongelados CON su causa.\n%s",
		strings.Join(fallos, "\n"))

	// Contra-prueba del baseline: si una entrada congelada dejó de fallar, alguien la arregló y
	// hay que sacarla. Un baseline que lista lo que ya no está roto tapa un fallo nuevo con un
	// hueco viejo — es el mismo defecto que el guard de DOMAINSERV-244 vigila con sus fantasmas.
	var fantasmas []string
	for clave := range hallazgosCongelados {
		if !contiene(congeladosVistos, clave) {
			fantasmas = append(fantasmas, clave)
		}
	}
	sort.Strings(fantasmas)
	require.Empty(t, fantasmas,
		"hallazgosCongelados lista queries que YA preparan bien (o que se movieron de línea): "+
			"sacalas del baseline, o deja de describir el código y se vuelve un cajón\n%s",
		strings.Join(fantasmas, "\n"))

	// La cobertura se DECLARA, no se insinúa: si este guard callara cuántas quedaron afuera,
	// repetiría el defecto que vino a cerrar —un número tranquilizador sobre un subconjunto—.
	// Las no analizables son las que se arman con verbos de fmt: su texto final no existe
	// hasta el runtime, así que PREPARE no puede verlas y ninguna herramienta estática tampoco.
	t.Logf("cobertura del guard de SQL crudo: %d queries preparadas, %d NO analizables "+
		"(construidas con verbos de fmt, su texto final no existe en el fuente)",
		len(crudas), len(noAnalizables))
	for _, q := range noAnalizables {
		t.Logf("  no analizable: %s:%d", q.archivo, q.linea)
	}
}

// recolectarCrudas devuelve los literales SQL del módulo que se pueden preparar, y aparte los
// que no. Recorre TODO el .go que no sea test ni generado por sqlc: el hermano ya cubre esos.
func recolectarCrudas(t *testing.T) (preparables, noAnalizables []queryCruda) {
	t.Helper()

	raiz, err := filepath.Abs("../..")
	require.NoError(t, err)

	err = filepath.Walk(raiz, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		// los generados por sqlc los cubre el guard de DOMAINSERV-199, y los _test.go no
		// corren en producción
		if info.Name() == "query.sql.go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(raiz, path)

		// Un literal que participa de una concatenación es un FRAGMENTO: la sentencia real es la
		// suma, y preparar el pedazo devuelve "syntax error at end of input", que no dice nada
		// del código. Se marcan primero para excluirlos, porque ast.Inspect no da el padre.
		fragmentos := map[token.Pos]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			be, ok := n.(*ast.BinaryExpr)
			if !ok || be.Op != token.ADD {
				return true
			}
			ast.Inspect(be, func(m ast.Node) bool {
				if l, ok := m.(*ast.BasicLit); ok && l.Kind == token.STRING {
					fragmentos[l.Pos()] = true
				}
				return true
			})
			return true
		})

		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || fragmentos[lit.Pos()] {
				return true
			}
			sql, uerr := strconv.Unquote(lit.Value)
			if uerr != nil || !pareceSQLCrudo(sql) {
				return true
			}
			q := queryCruda{archivo: rel, linea: fset.Position(lit.Pos()).Line, sql: sql}
			if tieneVerboDeFmt(sql) {
				noAnalizables = append(noAnalizables, q)
				return true
			}
			preparables = append(preparables, q)
			return true
		})
		return nil
	})
	require.NoError(t, err)

	orden := func(s []queryCruda) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].archivo != s[j].archivo {
				return s[i].archivo < s[j].archivo
			}
			return s[i].linea < s[j].linea
		})
	}
	orden(preparables)
	orden(noAnalizables)
	return preparables, noAnalizables
}

// pareceSQLCrudo exige el verbo inicial Y una palabra clave estructural, porque solo el verbo
// da falsos positivos: un mensaje de error o una línea de documentación que empieza con
// "SELECT" no es una sentencia. Se pide además un largo mínimo para descartar fragmentos.
func pareceSQLCrudo(s string) bool {
	t := strings.ToUpper(strings.TrimSpace(s))
	if len(t) < 25 {
		return false
	}
	// las de sqlc arrancan con su comentario de nombre y las cubre el otro guard
	if strings.HasPrefix(t, "-- NAME:") {
		return false
	}
	empieza := false
	for _, verbo := range []string{"SELECT ", "INSERT INTO ", "UPDATE ", "DELETE FROM ", "WITH "} {
		if strings.HasPrefix(t, verbo) {
			empieza = true
			break
		}
	}
	if !empieza {
		return false
	}
	for _, estructura := range []string{" FROM ", " INTO ", " SET ", " WHERE ", " AS ("} {
		if strings.Contains(t, estructura) {
			return true
		}
	}
	return false
}

func contiene(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// tieneVerboDeFmt detecta el SQL que se completa en runtime. No se puede preparar —el texto
// del fuente no es el que se ejecuta— y por eso se cuenta aparte en vez de silenciarse.
func tieneVerboDeFmt(s string) bool {
	for _, verbo := range []string{"%s", "%d", "%v", "%q", "%w"} {
		if strings.Contains(s, verbo) {
			return true
		}
	}
	return false
}
