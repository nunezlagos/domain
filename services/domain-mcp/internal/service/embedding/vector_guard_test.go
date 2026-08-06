package embedding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-157: el vector CERO de un chunk no se persiste — se guarda NULL.
//
// El guard de observaciones (observation/vector_literal_test.go) cubría solo esa tabla.
// knowledge_chunks tenía el mismo agujero por otra vía: el insert verificaba
// `len(embeds[i]) > 0`, y el NopEmbedder devuelve un slice de N ceros, no un slice
// vacío. Pasaba el largo y quedaba persistido como embedding legítimo.
//
// Importa por lo mismo que en observaciones: el CTE de búsqueda filtra por
// `embedding IS NOT NULL` sin mirar la norma (knowledge/sql/query.sql), así que los
// ceros no molestan mientras el embedder está roto y pasan a competir con los
// embeddings reales —a la misma distancia de cualquier query— el día que se arregle.
//
// El test se mudó desde internal/service/knowledge junto con la función
// (DOMAINSERV-227): ahí ya no quedaba escritura de embeddings que proteger.
func TestVectorOrNil_VectorCero_DevuelveNilParaQueSeGuardeNULL(t *testing.T) {
	assert.Nil(t, VectorOrNil(make([]float32, 1024)),
		"un vector de ceros no es un embedding: debe persistirse como NULL")
	assert.Nil(t, VectorOrNil([]float32{0, 0, 0}))
}

func TestVectorOrNil_VectorVacio_DevuelveNil(t *testing.T) {
	assert.Nil(t, VectorOrNil(nil))
	assert.Nil(t, VectorOrNil([]float32{}))
}

// El otro lado del invariante: un guard demasiado celoso apagaría la búsqueda
// semántica en silencio, que es el mismo modo de falla que este ticket combate.
func TestVectorOrNil_VectorReal_SeConserva(t *testing.T) {
	got := VectorOrNil([]float32{0.1, -0.5, 0})

	require.NotNil(t, got)
	assert.Equal(t, []float32{0.1, -0.5, 0}, got.Slice())
}

// Un solo componente no nulo alcanza: el guard mira si TODOS son cero.
func TestVectorOrNil_UnSoloValorNoNulo_NoSeDescarta(t *testing.T) {
	v := make([]float32, 1024)
	v[1023] = 0.0001

	assert.NotNil(t, VectorOrNil(v))
}
