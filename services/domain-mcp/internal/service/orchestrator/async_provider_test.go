package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/orchestrator/modes"
)

type asyncStubProvider struct{ name string }

func (p asyncStubProvider) Name() string { return p.name }
func (asyncStubProvider) Complete(context.Context, llm.CompletionOptions) (*llm.Response, error) {
	return &llm.Response{Content: "{}"}, nil
}
func (asyncStubProvider) CompleteStream(context.Context, llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

func TestService_ResolveAsyncProvider_ModelWithRegisteredProvider_ReturnsIt(t *testing.T) {
	const model = "gpt-4o-mini"
	name := modes.ProviderForModel(model)
	f := llm.NewFactory()
	f.Register(name, asyncStubProvider{name: name})

	s := &Service{LLM: f}
	p, err := s.resolveAsyncProvider(model)
	require.NoError(t, err)
	require.Equal(t, name, p.Name())
}

func TestService_ResolveAsyncProvider_UnregisteredProvider_FallsBackToDefault(t *testing.T) {
	f := llm.NewFactory()
	f.Register("opencode", asyncStubProvider{name: "opencode"})
	f.SetDefault("opencode", "")

	s := &Service{LLM: f}
	p, err := s.resolveAsyncProvider("claude-sonnet-5")
	require.NoError(t, err)
	require.Equal(t, "opencode", p.Name(), "modelo sin provider registrado cae al default")
}

func TestService_ResolveAsyncProvider_NoProviderNoDefault_ReturnsErrLLMFactoryRequired(t *testing.T) {
	f := llm.NewFactory()
	s := &Service{LLM: f}
	_, err := s.resolveAsyncProvider("claude-sonnet-5")
	require.ErrorIs(t, err, ErrLLMFactoryRequired)
}
