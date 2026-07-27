package skill

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-157: el vector CERO de una skill no se persiste — se guarda NULL.
//
// skills era el peor de los cuatro caminos de escritura: no tenía NINGÚN guard
// (ni siquiera el de largo que sí tenían los chunks), así que el slice de ceros
// del NopEmbedder entraba directo. Y skills tampoco estaba en backfillTargets,
// de modo que sus ceros no se regeneraban ni corriendo el backfill: quedaban
// para siempre.
//
// Son DOS los puntos de escritura: SkillCreate y SkillUpdateWithEmbedding. El
// tercer pgvector.NewVector del archivo (en SkillSearchHybridWithVector) es el
// vector de la QUERY, no una escritura, y ya está cubierto por el guard
// useVector — tocarlo habría roto la búsqueda sin arreglar nada.
func TestEmbeddingOrNil_VectorCero_DevuelveNilParaQueSeGuardeNULL(t *testing.T) {
	assert.Nil(t, embeddingOrNil(make([]float32, 1024)),
		"un vector de ceros no es un embedding: debe persistirse como NULL")
	assert.Nil(t, embeddingOrNil([]float32{0, 0, 0}))
}

func TestEmbeddingOrNil_VectorVacio_DevuelveNil(t *testing.T) {
	assert.Nil(t, embeddingOrNil(nil))
	assert.Nil(t, embeddingOrNil([]float32{}))
}

func TestEmbeddingOrNil_VectorReal_SeConserva(t *testing.T) {
	got := embeddingOrNil([]float32{0.1, -0.5, 0})

	require.NotNil(t, got)
	assert.Equal(t, []float32{0.1, -0.5, 0}, got.Slice())
}

func TestEmbeddingOrNil_UnSoloValorNoNulo_NoSeDescarta(t *testing.T) {
	v := make([]float32, 1024)
	v[1023] = 0.0001

	assert.NotNil(t, embeddingOrNil(v))
}
