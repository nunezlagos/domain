package analysis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

// recordingProvider captura el SystemPrompt recibido para verificar qué
// prompt usó el servicio (const vs loader).
type recordingProvider struct {
	gotSystemPrompt string
	response        string
}

func (p *recordingProvider) Name() string { return "recording" }
func (p *recordingProvider) Complete(_ context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	p.gotSystemPrompt = opts.SystemPrompt
	return &llm.Response{Content: p.response, Model: "stub"}, nil
}
func (p *recordingProvider) CompleteStream(_ context.Context, _ llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

func newServiceWith(prov llm.Provider, loader func(context.Context) (string, error)) *Service {
	f := llm.NewFactory()
	f.Register("", prov)
	return &Service{LLM: f, PromptLoader: loader}
}

func TestExplore_PromptLoader_OverridesConst(t *testing.T) {
	prov := &recordingProvider{response: "# Análisis\nok"}
	const custom = "PROMPT EDITADO DESDE EL DASHBOARD — analizá todo en una línea"
	s := newServiceWith(prov, func(_ context.Context) (string, error) {
		return custom, nil
	})
	_, err := s.explore(context.Background(), Input{RawText: "analizá X"})
	require.NoError(t, err)
	require.Equal(t, custom, prov.gotSystemPrompt, "el loader debe pisar el const")
}

func TestExplore_PromptLoader_EmptyFallsBackToConst(t *testing.T) {
	prov := &recordingProvider{response: "# Análisis\nok"}
	s := newServiceWith(prov, func(_ context.Context) (string, error) {
		return "   ", nil
	})
	_, err := s.explore(context.Background(), Input{RawText: "x"})
	require.NoError(t, err)
	require.Equal(t, DefaultAnalysisSystemPrompt, prov.gotSystemPrompt)
}

func TestExplore_PromptLoader_ErrorFallsBackToConst(t *testing.T) {
	prov := &recordingProvider{response: "# Análisis\nok"}
	s := newServiceWith(prov, func(_ context.Context) (string, error) {
		return "", context.DeadlineExceeded
	})
	_, err := s.explore(context.Background(), Input{RawText: "x"})
	require.NoError(t, err)
	require.Equal(t, DefaultAnalysisSystemPrompt, prov.gotSystemPrompt)
}

func TestExplore_NilLoader_UsesConst(t *testing.T) {
	prov := &recordingProvider{response: "# Análisis\nok"}
	s := newServiceWith(prov, nil)
	_, err := s.explore(context.Background(), Input{RawText: "x"})
	require.NoError(t, err)
	require.Equal(t, DefaultAnalysisSystemPrompt, prov.gotSystemPrompt)
}
