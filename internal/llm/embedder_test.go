package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNopEmbedder_ReturnsZero(t *testing.T) {
	e := NopEmbedder{}
	v, err := e.Embed(context.Background(), "hello")
	require.NoError(t, err)
	require.Equal(t, 1536, len(v))
	require.True(t, IsZero(v))
}

func TestFakeEmbedder_Deterministic(t *testing.T) {
	e := FakeEmbedder{Dim: 16}
	ctx := context.Background()
	a, _ := e.Embed(ctx, "hello world")
	b, _ := e.Embed(ctx, "hello world")
	require.Equal(t, a, b, "mismo text → mismo vector (determinístico)")
}

func TestFakeEmbedder_DifferentTextDifferentVector(t *testing.T) {
	e := FakeEmbedder{Dim: 16}
	ctx := context.Background()
	a, _ := e.Embed(ctx, "hello")
	b, _ := e.Embed(ctx, "world")
	require.NotEqual(t, a, b, "texts distintos producen vectors distintos")
}

func TestFakeEmbedder_UnitNorm(t *testing.T) {
	e := FakeEmbedder{Dim: 32}
	v, _ := e.Embed(context.Background(), "test")
	var sumSq float64
	for _, x := range v {
		sumSq += float64(x) * float64(x)
	}
	require.InDelta(t, 1.0, sumSq, 1e-4, "vector debe ser unit-norm")
}

func TestIsZero(t *testing.T) {
	require.True(t, IsZero([]float32{0, 0, 0}))
	require.False(t, IsZero([]float32{0, 0.1, 0}))
}
