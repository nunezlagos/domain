package knowledge

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-182: SearchHybrid, SearchBm25 y GetDoc no filtraban por ningún eje de
// tenant. JOINeaban knowledge_docs solo para `deleted_at IS NULL`, así que
// domain_knowledge_search devolvía chunks de cualquier proyecto. El agujero venía de
// la migración 000142, que eliminó organization_id de todas las tablas del schema y
// dejó el Go colgando: el handler seguía pasando un orgID que el SQL ya no podía usar.
//
// El eje elegido es project_id, que sobrevivió a la 000142 y es NOT NULL en
// knowledge_docs. Este guard mira la FUENTE porque las queries las genera sqlc y no
// hay función pura que testear: es lo único que atrapa la regresión sin Docker. El
// test de aislamiento real vive en project_scope_integration_test.go.

// queriesConScopeDeProyecto son las queries que leen contenido de knowledge y por lo
// tanto tienen que filtrar por proyecto. ListDocsByProject ya lo hacía y sirve de
// referencia del patrón correcto.
var queriesConScopeDeProyecto = []string{
	"SearchHybrid",
	"SearchBm25",
	"GetDoc",
	"ListDocsByProject",
}

func leerQuerySQL(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("sql/query.sql")
	require.NoError(t, err, "sin el .sql el guard no puede verificar nada")
	return string(b)
}

// bloqueDeQuery devuelve el cuerpo de una query sqlc, desde su `-- name:` hasta el
// próximo. Verificar sobre el archivo entero no serviría: cualquier mención de
// project_id en otra query daría un falso verde.
func bloqueDeQuery(t *testing.T, sql, nombre string) string {
	t.Helper()
	inicio := strings.Index(sql, "-- name: "+nombre+" ")
	require.NotEqual(t, -1, inicio, "la query %s no existe en sql/query.sql", nombre)
	resto := sql[inicio+len("-- name: "+nombre):]
	if fin := strings.Index(resto, "-- name: "); fin != -1 {
		return resto[:fin]
	}
	return resto
}

// predicadoDeProyecto es el filtro real, no la mención. Buscar solo "project_id" da
// falso verde: las queries de búsqueda ya SELECCIONAN d.project_id en su salida, así
// que la palabra aparece sin que exista ningún filtro. La primera versión de este
// guard cayó justo en eso.
const predicadoDeProyecto = "project_id = sqlc.arg('project_id')"

// Una query de lectura sin filtro de proyecto devuelve contenido de otros tenants. No
// es una preferencia de estilo: es el bug que este ticket vino a cerrar.
func TestQuerySQL_TodaQueryDeLectura_FiltraPorProjectID(t *testing.T) {
	sql := leerQuerySQL(t)

	for _, nombre := range queriesConScopeDeProyecto {
		bloque := bloqueDeQuery(t, sql, nombre)
		require.Contains(t, bloque, predicadoDeProyecto,
			"la query %s lee contenido de knowledge sin filtrar por proyecto: devuelve filas de otros tenants", nombre)
	}
}

// SearchHybrid tiene DOS CTEs que arman los candidatos (bm25 y vec) y el filtro tiene
// que estar en los dos. Con el filtro solo en el SELECT final, los CTEs igual
// escanearían todos los proyectos y el LIMIT de candidatos se gastaría en filas
// ajenas: el leak se cerraría de cara al usuario pero la query seguiría siendo
// cross-tenant por dentro, y devolvería menos resultados propios de los que hay.
func TestSearchHybrid_LosDosCTEsDeCandidatos_FiltranPorProjectID(t *testing.T) {
	bloque := bloqueDeQuery(t, leerQuerySQL(t), "SearchHybrid")

	inicioVec := strings.Index(bloque, "vec AS (")
	require.NotEqual(t, -1, inicioVec, "el CTE vec desapareció: revisar la query antes de tocar el guard")

	cteBm25, cteVec := bloque[:inicioVec], bloque[inicioVec:]
	require.Contains(t, cteBm25, predicadoDeProyecto, "el CTE bm25 de SearchHybrid no filtra por proyecto")
	require.Contains(t, cteVec, predicadoDeProyecto, "el CTE vec de SearchHybrid no filtra por proyecto")
}

// GetDoc por id sin filtro es un IDOR por UUID: quien adivine o conserve un id lee el
// doc de otro proyecto. Es el mismo modo de falla que DOMAINSERV-112 en
// file_attachments.
func TestGetDoc_FiltraPorProjectID_NoSoloPorID(t *testing.T) {
	bloque := bloqueDeQuery(t, leerQuerySQL(t), "GetDoc")
	require.Contains(t, bloque, predicadoDeProyecto,
		"GetDoc resuelve por id sin scope: un id de otro proyecto devuelve su documento")
}
