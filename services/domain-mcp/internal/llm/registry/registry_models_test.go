package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func usageDePrueba() llm.Usage {
	return llm.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
}

// DOMAINSERV-162: los modelos vigentes tienen que estar cotizados.
//
// El catálogo tenía tres entradas Anthropic y ninguna de las dos generaciones
// vigentes: ni Opus 5 ni Sonnet 5. Un modelo ausente no falla ruidosamente —
// CostUSD devuelve (0, ErrModelNotFound) y el runner descartaba ese error—, así
// que las corridas quedaban registradas como gratis.
//
// Este test no verifica PRECIOS: un assert de "opus-5 cuesta 5/25" solo repite
// el literal de al lado y no prueba nada. Verifica PRESENCIA, que es el
// invariante que sí se rompe solo cuando sale un modelo nuevo.
func TestRegistry_Get_ModelosAnthropicVigentes_EstanCotizados(t *testing.T) {
	r := New()
	ctx := context.Background()

	vigentes := []string{
		"claude-opus-5",
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-sonnet-5",
		"claude-sonnet-4-6",
		"claude-haiku-4-5-20251001",
		"claude-fable-5",
	}

	for _, id := range vigentes {
		m, err := r.Get(ctx, "anthropic", id)
		require.NoError(t, err, "%s no está en el registry: sus corridas se registran con costo 0", id)
		require.NotNil(t, m.InputPerMillion, "%s sin precio de input", id)
		require.NotNil(t, m.OutputPerMillion, "%s sin precio de output", id)
		assert.Greater(t, *m.InputPerMillion, 0.0, "%s: un precio en 0 es indistinguible de gratis", id)
		assert.Greater(t, *m.OutputPerMillion, 0.0, "%s: un precio en 0 es indistinguible de gratis", id)
	}
}

// El contexto declarado importa tanto como el precio: se usa para decidir si un
// prompt entra. Declarar 200K en un modelo de 1M subdimensiona los lotes.
func TestRegistry_Get_ContextoDeLaFamilia5_EsDeUnMillon(t *testing.T) {
	r := New()
	ctx := context.Background()

	for _, id := range []string{"claude-opus-5", "claude-sonnet-5", "claude-sonnet-4-6", "claude-opus-4-7"} {
		m, err := r.Get(ctx, "anthropic", id)
		require.NoError(t, err)
		require.NotNil(t, m.ContextSize, "%s sin contexto declarado", id)
		assert.Equal(t, 1_000_000, *m.ContextSize, "%s: contexto mal declarado", id)
	}
}

// Haiku es la excepción de la familia: 200K, no 1M.
func TestRegistry_Get_Haiku_MantieneSus200K(t *testing.T) {
	m, err := New().Get(context.Background(), "anthropic", "claude-haiku-4-5-20251001")

	require.NoError(t, err)
	require.NotNil(t, m.ContextSize)
	assert.Equal(t, 200_000, *m.ContextSize)
}

// El otro lado del contrato: un modelo que de verdad no existe tiene que dar
// ErrModelNotFound, no un cero silencioso. Es lo que el consumidor necesita
// para poder distinguir "no cotizado" de "gratis".
func TestRegistry_CostUSD_ModeloDesconocido_DevuelveErrModelNotFound(t *testing.T) {
	_, err := New().CostUSD(context.Background(), "anthropic", "claude-inexistente-9", usageDePrueba())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrModelNotFound)
}
