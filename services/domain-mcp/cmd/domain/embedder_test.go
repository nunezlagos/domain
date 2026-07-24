package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func isNop(e llm.Embedder) bool {
	_, ok := e.(llm.NopEmbedder)
	return ok
}

// mentirosoEmbedder declara una dimensión y produce otra — el caso que el guard
// viejo no atrapaba, porque solo miraba Dimensions(). Es el comportamiento REAL
// de los 3 providers antes de DOMAINSERV-80 H2: todos devolvían la constante
// del esquema (1536) sin mirar el modelo configurado.
type mentirosoEmbedder struct {
	declarada int
	real      int
	err       error
}

func (m mentirosoEmbedder) Dimensions() int { return m.declarada }
func (m mentirosoEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return make([]float32, m.real), nil
}
func (m mentirosoEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i], _ = m.Embed(ctx, texts[i])
	}
	return out, nil
}

func TestValidateDim_EmbedderDeclaraBienPeroProduceOtra_DegradaANoop(t *testing.T) {
	e := validateDim(mentirosoEmbedder{declarada: embeddingDim, real: 768}, testLogger())
	require.True(t, isNop(e),
		"un embedder que DECLARA la dim del esquema pero produce 768 debe degradar: "+
			"escribir ese vector reventaría el INSERT contra la columna vector(N)")
}

func TestValidateDim_DimensionRealCoincide_MantieneEmbedder(t *testing.T) {
	e := validateDim(mentirosoEmbedder{declarada: embeddingDim, real: embeddingDim}, testLogger())
	require.False(t, isNop(e))
}

func TestValidateDim_ProbeFalla_DegradaANoop(t *testing.T) {
	e := validateDim(mentirosoEmbedder{declarada: embeddingDim, err: errors.New("provider caído")}, testLogger())
	require.True(t, isNop(e),
		"si no se puede medir la dimensión real, fail-closed: mejor FTS que corromper escrituras")
}

func TestValidateDim_Nop_SeMantiene(t *testing.T) {
	require.True(t, isNop(validateDim(llm.NopEmbedder{}, testLogger())))
}

// Regresión de prod: llm.NopEmbedder tiene su PROPIO default hardcodeado (1536,
// "to match migration 000006"). Al bajar embeddingDim a 1024 quedaron dos fuentes
// de verdad, y como el noop igual escribe su vector cero en la columna, cada
// INSERT reventó con "expected 1024 dimensions, not 1536". El guard no lo vio
// porque solo mira embedders NO-noop.
func TestChooseEmbedder_Noop_ProduceVectorDeLaDimensionDelEsquema(t *testing.T) {
	for _, provider := range []string{"noop", "", "gibberish"} {
		t.Setenv("DOMAIN_EMBEDDING_PROVIDER", provider)
		e := chooseEmbedder(testLogger())
		require.True(t, isNop(e))

		v, err := e.Embed(context.Background(), "x")
		require.NoError(t, err)
		require.Len(t, v, embeddingDim,
			"el vector cero del noop se escribe igual en la columna: si no mide "+
				"embeddingDim, todo INSERT falla (provider=%q)", provider)
		require.Equal(t, embeddingDim, e.Dimensions(), "provider=%q", provider)
	}
}

// Los providers sin key caen a noop; ese noop también tiene que medir bien.
func TestChooseEmbedder_SinKey_CaeANoopConLaDimensionCorrecta(t *testing.T) {
	for _, c := range []struct{ provider, keyEnv string }{
		{"openai", "DOMAIN_OPENAI_API_KEY"},
		{"voyage", "DOMAIN_VOYAGE_API_KEY"},
	} {
		t.Setenv("DOMAIN_EMBEDDING_PROVIDER", c.provider)
		t.Setenv(c.keyEnv, "")
		e := chooseEmbedder(testLogger())
		require.True(t, isNop(e), "provider=%q", c.provider)

		v, err := e.Embed(context.Background(), "x")
		require.NoError(t, err)
		require.Len(t, v, embeddingDim, "provider=%q", c.provider)
	}
}

func TestValidateDim_FakeConDimDelEsquema_SeMantiene(t *testing.T) {
	require.False(t, isNop(validateDim(llm.FakeEmbedder{Dim: embeddingDim}, testLogger())))
}

func TestValidateDim_FakeConOtraDim_DegradaANoop(t *testing.T) {
	require.True(t, isNop(validateDim(llm.FakeEmbedder{Dim: 8}, testLogger())))
}

// Los providers reales necesitan red para el probe. Sin conectividad el guard
// degrada a noop, que es el comportamiento buscado (fail-closed) y lo que hace
// falta afirmar: antes estos tests fijaban 1536 como "dim de voyage/ollama",
// consagrando el bug que este cambio corrige.
func TestChooseEmbedder_VoyageSinConectividad_DegradaANoop(t *testing.T) {
	probeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { probeTimeout = defaultProbeTimeout })
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "voyage")
	t.Setenv("DOMAIN_VOYAGE_API_KEY", "vk-test-invalida")
	require.True(t, isNop(chooseEmbedder(testLogger())))
}

func TestChooseEmbedder_OllamaSinConectividad_DegradaANoop(t *testing.T) {
	probeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { probeTimeout = defaultProbeTimeout })
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "ollama")
	t.Setenv("DOMAIN_OLLAMA_URL", "http://127.0.0.1:1")
	require.True(t, isNop(chooseEmbedder(testLogger())))
}

func TestChooseEmbedder_VoyageNoKey_FallsBackToNoop(t *testing.T) {
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "voyage")
	t.Setenv("DOMAIN_VOYAGE_API_KEY", "")
	require.True(t, isNop(chooseEmbedder(testLogger())))
}

func TestChooseEmbedder_Unknown_FallsBackToNoop(t *testing.T) {
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "gibberish")
	require.True(t, isNop(chooseEmbedder(testLogger())))
}

func TestChooseEmbedder_Noop_NoHaceProbe(t *testing.T) {
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "noop")
	require.True(t, isNop(chooseEmbedder(testLogger())))
}
