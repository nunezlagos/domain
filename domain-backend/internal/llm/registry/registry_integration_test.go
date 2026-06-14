//go:build integration

package registry_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/registry"
	dmigrate "nunezlagos/domain/internal/migrate"
)

func setup(t *testing.T) (*registry.Registry, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	r := &registry.Registry{Pool: pool}
	return r, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
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
	// claude-sonnet-4-6: 3 USD/M input + 15 USD/M output
	// usage: 1M input + 100K output → 3 + 1.5 = 4.5 USD
	cost, err := r.CostUSD(ctx, "anthropic", "claude-sonnet-4-6", llm.Usage{
		PromptTokens: 1_000_000, CompletionTokens: 100_000,
	})
	require.NoError(t, err)
	require.InDelta(t, 4.5, cost, 0.001)
}

func TestRegistry_CostUSD_OllamaFree(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	cost, err := r.CostUSD(context.Background(), "ollama", "llama3.1", llm.Usage{
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
	require.True(t, len(models) >= 8, "al menos 8 modelos seedeados")
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

// Sabotaje: cache TTL no causa lost updates si admin actualiza precio.
func TestSabotage_Registry_RefreshAfterUpdate(t *testing.T) {
	r, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	// Read 1: cache fresh
	_, err := r.Get(ctx, "openai", "gpt-4o")
	require.NoError(t, err)

	// Admin actualiza precio
	_, err = r.Pool.Exec(ctx,
		`UPDATE model_registry SET input_per_million = 99.99 WHERE provider='openai' AND model='gpt-4o'`)
	require.NoError(t, err)

	// Sin Refresh explicit, cache devuelve viejo. Llamamos Refresh:
	require.NoError(t, r.Refresh(ctx))
	m, _ := r.Get(ctx, "openai", "gpt-4o")
	require.Equal(t, 99.99, *m.InputPerMillion)
}
