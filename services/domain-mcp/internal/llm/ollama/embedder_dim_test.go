package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-157: Dimensions() estaba hardcodeado en 1536 mientras el esquema
// pgvector es vector(1024) desde la migración 000275 (bge-m3).
//
// El daño no era solo informativo: embed() rellena el vector hasta
// e.Dimensions(), así que el probe de arranque medía SIEMPRE 1536 sin importar
// qué devolviera el modelo. validateDim comparaba contra 1024, no coincidía, y
// degradaba a noop — de forma determinista, en cada arranque.
//
// Eso dejó la búsqueda semántica apagada en producción con este log:
//
//	embedder: ollama  model=bge-m3
//	embedder: la dimensión producida no coincide con el esquema pgvector; degradando a noop
//
// El mensaje era correcto y la causa estaba acá: no era el modelo ni el
// esquema, era esta constante.
func TestEmbedder_Dimensions_DerivaDelModeloYNoDeUnaConstante(t *testing.T) {
	e := NewEmbedder("bge-m3")

	assert.Equal(t, 1024, e.Dimensions(),
		"bge-m3 produce 1024; con 1536 el probe degrada el embedder a noop en cada arranque")
}

// nomic-embed-text es el default histórico de NewEmbedder y sí produce 768. Si
// se le devolviera la dimensión de bge-m3 se rompería al revés.
func TestEmbedder_Dimensions_ModeloConocidoDistinto_DevuelveLaSuya(t *testing.T) {
	assert.Equal(t, 768, NewEmbedder("nomic-embed-text").Dimensions())
	assert.Equal(t, 768, NewEmbedder("").Dimensions(), "el default es nomic-embed-text")
}

// El invariante que importa de verdad: lo que embed() devuelve tiene que tener
// el LARGO REAL de lo que mandó el provider. Rellenar hasta una constante es lo
// que hacía que el probe midiera la constante en vez del modelo, y con eso
// ningún guard podía detectar un desalineo real.
func TestEmbedder_Embed_DevuelveElLargoRealDelProvider_NoUnaConstante(t *testing.T) {
	const dimReal = 1024
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vals := make([]float64, dimReal)
		for i := range vals {
			vals[i] = 0.5
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float64{vals}}))
	}))
	defer srv.Close()

	e := NewEmbedder("bge-m3")
	e.BaseURL = srv.URL

	v, err := e.Embed(context.Background(), "hola")

	require.NoError(t, err)
	assert.Len(t, v, dimReal, "el largo debe venir del provider, no de Dimensions()")
}

// Y el caso que el bug ocultaba: si el provider devuelve OTRA dimensión, el
// llamador tiene que poder verla para degradar con criterio. Rellenar a una
// constante hacía que un modelo equivocado pasara desapercibido.
func TestEmbedder_Embed_ProviderDevuelveOtraDimension_NoSeEnmascara(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float64{make([]float64, 384)}}))
	}))
	defer srv.Close()

	e := NewEmbedder("bge-m3")
	e.BaseURL = srv.URL

	v, err := e.Embed(context.Background(), "hola")

	require.NoError(t, err)
	assert.Len(t, v, 384, "un desalineo real tiene que ser visible para el guard de validateDim")
}
