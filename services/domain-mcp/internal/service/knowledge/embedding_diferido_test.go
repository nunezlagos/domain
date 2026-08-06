package knowledge

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-227: Save NO puede llamar al embedder. El costo era ~2,3s por chunk contra
// Ollama —un POST por chunk, porque EmbedBatch de ollama es un for secuencial— y estaba
// entero dentro del request, así que era lineal en la cantidad de chunks: un documento
// de 157KB agotaba el timeout del cliente MCP antes de recibir el ack.
//
// El guard lee el código y no mockea el embedder a propósito. Un test con espía verifica
// que ESA llamada no ocurre; este verifica que no ocurre NINGUNA, incluida la que alguien
// agregue mañana por otra vía. Es la misma técnica que knowledge_rls_test.go.
func TestSave_NoEmbebeDentroDelRequest(t *testing.T) {
	cuerpo := cuerpoDeFuncion(t, "internal/service/knowledge/service.go", "func (s *Service) Save(")

	assert.NotContains(t, cuerpo, "EmbedBatch(",
		"Save volvió a embebir dentro del request: es el bug de DOMAINSERV-227")
	assert.NotContains(t, cuerpo, "s.Embedder.Embed(",
		"Save volvió a embebir dentro del request: es el bug de DOMAINSERV-227")
}

// El otro lado del cambio: los chunks se persisten con embedding NULL, que es el estado
// válido de "pendiente". Escribir un vector de ceros lo haría indistinguible de un
// embedding legítimo y lo sacaría del barrido para siempre (DOMAINSERV-157).
func TestSave_PersisteLosChunksConEmbeddingEnNil(t *testing.T) {
	cuerpo := cuerpoDeFuncion(t, "internal/service/knowledge/service.go", "func (s *Service) Save(")

	require.Contains(t, cuerpo, "InsertChunkParams{")
	assert.Contains(t, cuerpo, "Embedding:      nil",
		"el chunk pendiente va con embedding NULL, no con un vector de ceros")
}

// Criterio 4 de DOMAINSERV-227: un doc con embeddings todavía pendientes tiene que
// seguir siendo encontrable por texto, no devolver vacío.
//
// Lo que lo garantiza es el FULL OUTER JOIN entre los dos CTEs: el brazo `vec` filtra
// `embedding IS NOT NULL`, pero el brazo `bm25` NO, así que un chunk sin vector entra al
// resultado igual. Si alguien cambiara el join a INNER —o agregara el filtro de embedding
// al brazo bm25— la ingesta diferida se volvería invisible hasta que el worker corra, y
// el síntoma sería "busqué y no está" sin ningún error.
func TestSearchHybrid_ChunkSinEmbedding_EntraPorElBrazoBm25(t *testing.T) {
	sql := leerArchivo(t, "internal/service/knowledge/sql/query.sql")
	hybrid := recorte(t, sql, "-- name: SearchHybrid :many", "-- name: SearchBm25")

	assert.Contains(t, hybrid, "FULL OUTER JOIN",
		"con un INNER JOIN los chunks sin embedding desaparecerían de la búsqueda")

	bm25 := recorte(t, hybrid, "WITH bm25 AS (", "vec AS (")
	assert.NotContains(t, bm25, "embedding",
		"el brazo bm25 no debe mirar el embedding: es la rama que sostiene el fallback textual")
}

func leerArchivo(t *testing.T, rel string) string {
	t.Helper()
	// los tests corren desde el dir del package: subir hasta la raíz del módulo
	b, err := os.ReadFile("../../../" + rel)
	require.NoError(t, err, "no pude leer %s", rel)
	return string(b)
}

func recorte(t *testing.T, texto, desde, hasta string) string {
	t.Helper()
	i := strings.Index(texto, desde)
	require.GreaterOrEqual(t, i, 0, "no encontré el marcador %q", desde)
	resto := texto[i:]
	if j := strings.Index(resto, hasta); j > 0 {
		return resto[:j]
	}
	return resto
}

// cuerpoDeFuncion devuelve el texto entre la firma y el próximo `\n}` a nivel de
// archivo. Alcanza para un guard de presencia de llamadas y evita traer go/parser.
func cuerpoDeFuncion(t *testing.T, rel, firma string) string {
	t.Helper()
	return recorte(t, leerArchivo(t, rel), firma, "\n}\n")
}
