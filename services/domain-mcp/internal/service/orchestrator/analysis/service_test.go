package analysis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

type stubProvider struct{ content string }

func (stubProvider) Name() string { return "stub" }
func (s stubProvider) Complete(context.Context, llm.CompletionOptions) (*llm.Response, error) {
	return &llm.Response{Content: s.content}, nil
}
func (stubProvider) CompleteStream(context.Context, llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

// TestExplore_UsesDefaultProvider_ReturnsContent verifica que explore resuelve
// el provider default del factory (opencode en prod). Antes usaba Get(""), que
// SIEMPRE fallaba → el análisis nunca corría (DOMAINSERV-65)
func TestExplore_UsesDefaultProvider_ReturnsContent(t *testing.T) {
	f := llm.NewFactory()
	f.Register("opencode", stubProvider{content: "# Análisis\ncontenido del cerebro"})
	f.SetDefault("opencode", "")

	s := &Service{LLM: f}
	out, err := s.explore(context.Background(), Input{RawText: "analizá X"})
	require.NoError(t, err)
	require.Contains(t, out, "contenido del cerebro")
}
