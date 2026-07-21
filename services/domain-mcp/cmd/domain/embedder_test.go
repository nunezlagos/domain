package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func isNop(e llm.Embedder) bool {
	_, ok := e.(llm.NopEmbedder)
	return ok
}

func TestChooseEmbedder_Voyage_ReturnsRealEmbedderWithSchemaDim(t *testing.T) {
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "voyage")
	t.Setenv("DOMAIN_VOYAGE_API_KEY", "vk-test")
	e := chooseEmbedder(testLogger())
	require.False(t, isNop(e), "voyage con key no debe caer a noop")
	require.Equal(t, 1536, e.Dimensions())
}

func TestChooseEmbedder_VoyageNoKey_FallsBackToNoop(t *testing.T) {
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "voyage")
	t.Setenv("DOMAIN_VOYAGE_API_KEY", "")
	require.True(t, isNop(chooseEmbedder(testLogger())))
}

func TestChooseEmbedder_Ollama_ReturnsRealEmbedder(t *testing.T) {
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "ollama")
	e := chooseEmbedder(testLogger())
	require.False(t, isNop(e))
	require.Equal(t, 1536, e.Dimensions())
}

func TestValidateDim_Mismatch_DegradesToNoop(t *testing.T) {
	e := validateDim(llm.FakeEmbedder{Dim: 8}, testLogger())
	require.True(t, isNop(e), "dim 8 != 1536 debe degradar a noop para no corromper el indice")
}

func TestValidateDim_Match_KeepsEmbedder(t *testing.T) {
	e := validateDim(llm.FakeEmbedder{}, testLogger()) // Dim 0 -> 1536
	require.False(t, isNop(e))
}

func TestChooseEmbedder_Unknown_FallsBackToNoop(t *testing.T) {
	t.Setenv("DOMAIN_EMBEDDING_PROVIDER", "gibberish")
	require.True(t, isNop(chooseEmbedder(testLogger())))
}
