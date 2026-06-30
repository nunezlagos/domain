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

func TestNopEmbedder_EmbedBatch(t *testing.T) {
	e := NopEmbedder{}
	vs, err := e.EmbedBatch(context.Background(), []string{"a", "b", "c"})
	require.NoError(t, err)
	require.Len(t, vs, 3)
	for _, v := range vs {
		require.Len(t, v, 1536)
		require.True(t, IsZero(v))
	}
}

func TestFakeEmbedder_EmbedBatch(t *testing.T) {
	e := FakeEmbedder{Dim: 8}
	ctx := context.Background()
	vs, err := e.EmbedBatch(ctx, []string{"hello", "world"})
	require.NoError(t, err)
	require.Len(t, vs, 2)
	for _, v := range vs {
		require.Len(t, v, 8)
	}
	require.NotEqual(t, vs[0], vs[1])
}

func TestTruncateText_Short(t *testing.T) {
	short := "hello world"
	got := TruncateText(short, 100)
	require.Equal(t, short, got)
}

func TestTruncateText_Long(t *testing.T) {
	long := repeatStr("hello world ", 200)
	got := TruncateText(long, 100)
	require.LessOrEqual(t, len(got), 100*4)
	require.Less(t, len(got), len(long))
}

func TestTruncateText_Zero(t *testing.T) {
	require.Equal(t, "", TruncateText("anything", 0))
	require.Equal(t, "", TruncateText("anything", -1))
}

func repeatStr(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestVectorLiteral(t *testing.T) {
	v := []float32{0.1, 0.2, 0.3}
	lit := VectorLiteral(v)
	require.Contains(t, lit, "'[0.1")
	require.Contains(t, lit, "::vector")
}

func TestCosineSimilaritySQL(t *testing.T) {
	sql := CosineSimilaritySQL("$1")
	require.Contains(t, sql, "<=>")
	require.Contains(t, sql, "::vector")
	require.Contains(t, sql, "AS similarity")
}

func TestCosineSimilarityOrder(t *testing.T) {
	sql := CosineSimilarityOrder("$1")
	require.Contains(t, sql, "<=>")
	require.Contains(t, sql, "ORDER BY")
}

// Sabotaje: vector vacío → literal no crash
func TestSabotage_VectorLiteral_Empty(t *testing.T) {
	lit := VectorLiteral(nil)
	require.Contains(t, lit, "::vector")
	lit = VectorLiteral([]float32{})
	require.Contains(t, lit, "::vector")
}

// Sabotaje: api key inválida debe fallar graceful
func TestSabotage_Embed_NoAPIKey(t *testing.T) {

	t.Skip("Requiere mock HTTP; probado en openai/provider_test.go")
}
