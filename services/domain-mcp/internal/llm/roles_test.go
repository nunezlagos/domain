package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type roleStub struct{ name string }

func (r roleStub) Name() string { return r.name }
func (r roleStub) Complete(context.Context, CompletionOptions) (*Response, error) {
	return &Response{}, nil
}
func (r roleStub) CompleteStream(context.Context, CompletionOptions) (<-chan StreamChunk, error) {
	return nil, nil
}

func TestProviderNameForModel_Prefixes_MapToRegisteredNames(t *testing.T) {
	require.Equal(t, "anthropic", ProviderNameForModel("claude-haiku-4-5"))
	require.Equal(t, "openai", ProviderNameForModel("gpt-4o"))
	require.Equal(t, "google", ProviderNameForModel("gemini-2.0"))
	require.Equal(t, "minimax", ProviderNameForModel("MiniMax-M3"))
	require.Equal(t, "ollama", ProviderNameForModel("llama3"))
}

func TestFactory_ProviderForRole_DefaultBinding_ResolvesProviderAndModel(t *testing.T) {
	f := NewFactory()
	f.Register("minimax", roleStub{name: "minimax"})
	p, model, err := f.ProviderForRole(RoleRerank)
	require.NoError(t, err)
	require.Equal(t, "minimax", p.Name())
	require.Equal(t, "MiniMax-M3", model)
}

func TestFactory_ProviderForRole_EnvOverride_UsesConfiguredProviderAndModel(t *testing.T) {
	t.Setenv("DOMAIN_LLM_RERANK_PROVIDER", "openai")
	t.Setenv("DOMAIN_LLM_RERANK_MODEL", "gpt-4o")
	f := NewFactory()
	f.Register("openai", roleStub{name: "openai"})
	p, model, err := f.ProviderForRole(RoleRerank)
	require.NoError(t, err)
	require.Equal(t, "openai", p.Name())
	require.Equal(t, "gpt-4o", model)
}

func TestFactory_ProviderForRole_PreferredAbsent_FallsBackToFactoryDefault(t *testing.T) {
	f := NewFactory()
	f.Register("openai", roleStub{name: "openai"})
	f.SetDefault("openai", "")
	// RoleRerank default provider = minimax (ausente) -> cae al default openai
	p, model, err := f.ProviderForRole(RoleRerank)
	require.NoError(t, err)
	require.Equal(t, "openai", p.Name())
	require.Equal(t, "", model) // vacio => el provider usa su modelo por default
}

func TestFactory_ProviderForRole_NoProviderNoDefault_Errors(t *testing.T) {
	f := NewFactory()
	_, _, err := f.ProviderForRole(RoleRerank)
	require.Error(t, err)
}
