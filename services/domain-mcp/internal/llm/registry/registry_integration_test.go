//go:build integration

package registry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/registry"
)




func setup(t *testing.T) (*registry.Registry, func()) {
	t.Helper()
	return registry.New(), func() {}
}

func TestRegistry_Seeds(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	m, err := r.Get(ctx, "anthropic", "claude-sonnet-4-6")
	require.NoError(t, err)
	require.NotNil(t, m.InputPerMillion)
	require.Equal(t, 3.0, *m.InputPerMillion)
	require.Equal(t, 15.0, *m.OutputPerMillion)
}

func TestRegistry_CostUSD(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()


	cost, err := r.CostUSD(ctx, "anthropic", "claude-sonnet-4-6", llm.Usage{
		PromptTokens: 1_000_000, CompletionTokens: 100_000,
	})
	require.NoError(t, err)
	require.InDelta(t, 4.5, cost, 0.001)
}

func TestRegistry_CostUSD_OllamaFree(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	cost, err := r.CostUSD(context.Background(), "ollama", "llama3.2:3b", llm.Usage{
		PromptTokens: 5000, CompletionTokens: 3000,
	})
	require.NoError(t, err)
	require.Equal(t, 0.0, cost, "ollama local debe ser sin costo")
}

func TestRegistry_NotFound(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	_, err := r.Get(context.Background(), "anthropic", "claude-no-existe")
	require.ErrorIs(t, err, registry.ErrModelNotFound)
}

func TestRegistry_List(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	models, err := r.List(context.Background())
	require.NoError(t, err)
	require.True(t, len(models) >= 8, "al menos 8 modelos en el catálogo")
}

// Embedding cost: solo input_per_million aplica (no hay output tokens).
func TestRegistry_CostUSD_EmbeddingModel(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	cost, err := r.CostUSD(context.Background(), "openai", "text-embedding-3-small",
		llm.Usage{PromptTokens: 1_000_000})
	require.NoError(t, err)
	require.InDelta(t, 0.02, cost, 0.0001)
}

// Refresh es no-op (catálogo estático en código); no debe romper.
func TestRegistry_RefreshNoop(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	require.NoError(t, r.Refresh(context.Background()))
}
