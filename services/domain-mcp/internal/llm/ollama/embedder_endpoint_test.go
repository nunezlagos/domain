package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// El Embedder usaba /api/embeddings, el endpoint legacy de ollama, que NO trunca:
// devuelve 500 "the input length exceeds the context length" ante cualquier texto
// que pase el num_ctx efectivo. Verificado contra ollama 0.32.3 en el VPS con un
// contenido real de 6128 chars: 500 en /api/embeddings, 200 con dims=1024 en
// /api/embed. Ese error subía como error del provider, backfillTable hacía
// continue, y 25 observaciones quedaban con embedding NULL de forma permanente.
func TestEmbedder_Embed_UsaElEndpointQueTrunca_NoElLegacy(t *testing.T) {
	var pedido string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedido = r.URL.Path
		require.NoError(t, json.NewEncoder(w).Encode(embedResp{Embeddings: [][]float64{make([]float64, 1024)}}))
	}))
	defer srv.Close()

	e := NewEmbedder("bge-m3")
	e.BaseURL = srv.URL

	_, err := e.Embed(context.Background(), "hola")

	require.NoError(t, err)
	assert.Equal(t, "/api/embed", pedido,
		"/api/embeddings es el endpoint legacy y no trunca: revienta con 500 en los textos largos")
}

// El endpoint nuevo recibe "input", no "prompt". Mandar el campo viejo hace que
// ollama embeddee la cadena vacía y devuelva un vector sin relación con el texto
// —peor que un error, porque no falla a la vista.
func TestEmbedder_Embed_MandaElCampoInput_NoPrompt(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &body))
		require.NoError(t, json.NewEncoder(w).Encode(embedResp{Embeddings: [][]float64{make([]float64, 1024)}}))
	}))
	defer srv.Close()

	e := NewEmbedder("bge-m3")
	e.BaseURL = srv.URL

	_, err := e.Embed(context.Background(), "el texto a embeddear")

	require.NoError(t, err)
	assert.Equal(t, "el texto a embeddear", body["input"])
	assert.NotContains(t, body, "prompt", "prompt es el campo del endpoint legacy")
}

// El shape de respuesta cambia con el endpoint: {"embeddings": [[...]]} anidado en
// vez de {"embedding": [...]} plano. Leer el campo viejo devolvería siempre vacío.
func TestEmbedder_Embed_ParseaElArrayAnidadoDeEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{"embeddings":[[0.1,0.2,0.3]]}`))
		require.NoError(t, err)
	}))
	defer srv.Close()

	e := NewEmbedder("bge-m3")
	e.BaseURL = srv.URL

	v, err := e.Embed(context.Background(), "hola")

	require.NoError(t, err)
	require.Len(t, v, 3)
	assert.InDelta(t, 0.1, v[0], 1e-6)
	assert.InDelta(t, 0.3, v[2], 1e-6)
}

// Sabotaje: si ollama responde 200 con la lista vacía, devolver un vector nulo
// dejaría que el llamador escriba NULL creyendo que embeddeó.
func TestEmbedder_Embed_ListaDeEmbeddingsVacia_DevuelveError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{"embeddings":[]}`))
		require.NoError(t, err)
	}))
	defer srv.Close()

	e := NewEmbedder("bge-m3")
	e.BaseURL = srv.URL

	_, err := e.Embed(context.Background(), "hola")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty embedding")
}

// El mensaje del provider tiene que llegar íntegro al llamador: es lo único que
// distingue "ollama está caído" de "este texto excede el contexto del modelo".
func TestEmbedder_Embed_ProviderDevuelve500_PropagaElMensaje(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"error":"the input length exceeds the context length"}`))
		require.NoError(t, err)
	}))
	defer srv.Close()

	e := NewEmbedder("bge-m3")
	e.BaseURL = srv.URL

	_, err := e.Embed(context.Background(), "hola")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "the input length exceeds the context length")
}
