package observation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// DOMAINSERV-157: el vector CERO no se persiste — se guarda NULL.
//
// El NopEmbedder devuelve un slice de N ceros, no un slice vacío. vectorLiteral
// solo devolvía "[]" para el vacío, así que los ceros se serializaban como
// "[0,0,...]" y terminaban en la columna. El INSERT los guarda porque su CASE
// solo mapea '[]' a NULL (sql/query.sql:7).
//
// Medido en prod el 2026-07-25: 44 observaciones con embedding NO NULL, las 44
// de norma 0. Cero embeddings útiles, indistinguibles de los buenos para
// cualquier consulta que mire `embedding IS NOT NULL`.
//
// Por qué importa aunque hoy la búsqueda degrade limpio: el guard que salva la
// situación (`useVector := !llm.IsZero(vec)`, service.go:276) mira el vector de
// la QUERY, no los almacenados. El CTE `vec` filtra por `embedding IS NOT NULL`
// sin mirar la norma. O sea que el día que el embedder se arregle y useVector
// pase a true, esos ceros entran a competir con los embeddings reales dando la
// misma distancia contra cualquier búsqueda. El daño no está ocurriendo: está
// esperando el arreglo.
func TestVectorLiteral_VectorCero_DevuelveVacioParaQueSeGuardeNULL(t *testing.T) {
	assert.Equal(t, "[]", vectorLiteral(make([]float32, 1024)),
		"un vector de ceros no es un embedding: debe persistirse como NULL, no como [0,0,...]")
	assert.Equal(t, "[]", vectorLiteral([]float32{0, 0, 0}))
}

func TestVectorLiteral_VectorVacio_DevuelveVacio(t *testing.T) {
	assert.Equal(t, "[]", vectorLiteral(nil))
	assert.Equal(t, "[]", vectorLiteral([]float32{}))
}

// El otro lado del invariante: un embedding REAL tiene que serializarse
// completo. Si el guard de ceros se pasara de celoso y descartara vectores
// válidos, apagaría la búsqueda semántica en silencio — que es justo el modo de
// falla que este ticket combate.
func TestVectorLiteral_VectorReal_SeSerializaCompleto(t *testing.T) {
	got := vectorLiteral([]float32{0.1, -0.5, 0})

	assert.Equal(t, "[0.1,-0.5,0]", got)
	assert.NotEqual(t, "[]", got, "un vector con al menos un valor no nulo SÍ es un embedding")
}

// Un solo componente distinto de cero alcanza para que el vector sea legítimo:
// el guard mira si TODOS son cero, no si alguno lo es.
func TestVectorLiteral_UnSoloValorNoNulo_NoSeDescarta(t *testing.T) {
	v := make([]float32, 1024)
	v[1023] = 0.0001

	got := vectorLiteral(v)

	assert.NotEqual(t, "[]", got)
	assert.True(t, strings.HasSuffix(got, ",0.0001]"), "se conserva el valor no nulo, cola: %s", got[len(got)-12:])
}
