//go:build integration

// Guard de DOMAINSERV-199: PREPARA todas las queries generadas por sqlc contra el
// schema real. Es la única capa que puede atrapar estos defectos —sqlc generate no
// valida la aridad de un INSERT ... SELECT, el compilador no ve strings embebidos, y
// PostgreSQL solo rechaza la sentencia al PREPARARLA— así que hace falta un motor vivo.
//
// El caso que lo motivó: RollupWeek declaraba 10 columnas en el target del INSERT y su
// SELECT proveía 9. Estuvo roto desde la migración 000181 sin que nadie lo notara,
// porque el cron que la llamaba estaba apagado por un flag.
//
// Va con el tag integration a propósito: ci-mcp.yml corre un job integration-tests con
// -tags=integration, así que este guard SÍ se ejecuta en CI.
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

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
)

type queryEmbebida struct {
	paquete string
	nombre  string
	sql     string
}

func TestSqlcQueries_TodasPreparanContraElSchemaReal(t *testing.T) {
	queries := recolectarQueries(t)
	require.NotEmpty(t, queries, "sin queries el guard no cubre nada: un error de setup es un fallo, nunca un skip")

	conn := conectarConMigracionesAplicadas(t)
	ctx := context.Background()

	var fallos []string
	for i, q := range queries {
		// el nombre del prepared statement tiene que ser único por sentencia
		if _, err := conn.Prepare(ctx, fmt.Sprintf("guard_%d", i), q.sql); err != nil {
			fallos = append(fallos, fmt.Sprintf("%s.%s: %v", q.paquete, q.nombre, err))
		}
	}

	t.Logf("DOMAINSERV-199: %d/%d queries preparan contra el schema real", len(queries)-len(fallos), len(queries))
	require.Empty(t, fallos,
		"hay queries que PostgreSQL rechaza al preparar — son bugs latentes esperando que alguien las ejecute:\n%s",
		strings.Join(fallos, "\n"))
}

// Sin esta cota, borrar los query.sql.go dejaría el guard en verde por vacío.
func TestSqlcQueries_ElGuardCubreTodosLosPaquetes(t *testing.T) {
	queries := recolectarQueries(t)

	paquetes := map[string]bool{}
	for _, q := range queries {
		paquetes[q.paquete] = true
	}

	generados := archivosGenerados(t)
	require.Len(t, paquetes, len(generados),
		"el guard tiene que cubrir los %d paquetes con query.sql.go, cubre %d", len(generados), len(paquetes))
}

// Las constantes salen del .go generado y no del sql/query.sql fuente: el generado ya
// tiene $1/$2 en lugar de sqlc.arg(), que es lo que PREPARE entiende.
func recolectarQueries(t *testing.T) []queryEmbebida {
	t.Helper()

	var out []queryEmbebida
	for _, archivo := range archivosGenerados(t) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, archivo, nil, 0)
		require.NoError(t, err, "parsear %s", archivo)

		paquete := filepath.Base(filepath.Dir(archivo))
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
					continue
				}
				lit, ok := vs.Values[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				sql, err := strconv.Unquote(lit.Value)
				if err != nil || !pareceSQL(sql) {
					continue
				}
				out = append(out, queryEmbebida{paquete: paquete, nombre: vs.Names[0].Name, sql: sql})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].paquete+out[i].nombre < out[j].paquete+out[j].nombre
	})
	return out
}

// sqlc antepone "-- name: X :kind" a cada query embebida; el resto de las constantes
// string del paquete no lo llevan
func pareceSQL(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "-- name:")
}

func archivosGenerados(t *testing.T) []string {
	t.Helper()

	raiz, err := filepath.Abs("../..")
	require.NoError(t, err)

	var out []string
	err = filepath.Walk(raiz, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "query.sql.go" {
			out = append(out, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, out, "no se encontró ningún query.sql.go: setup roto")
	sort.Strings(out)
	return out
}

func conectarConMigracionesAplicadas(t *testing.T) *pgx.Conn {
	t.Helper()
	ctx := context.Background()

	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, dmigrate.Up(dsn), "aplicar migraciones")

	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(ctx) })

	return conn
}
