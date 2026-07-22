package acp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	acpbridge "nunezlagos/domain/internal/agentbridge/acp"
	"nunezlagos/domain/internal/llm"
)

type fakeRunner struct {
	got    string
	reply  string
	err    error
	closed bool
}

func (f *fakeRunner) Prompt(_ context.Context, text string) (string, error) {
	f.got = text
	return f.reply, f.err
}
func (f *fakeRunner) Close() error { f.closed = true; return nil }

func providerWith(fr *fakeRunner, spawnErr error) *Provider {
	return &Provider{name: ProviderName, spawn: func(context.Context) (runner, error) {
		if spawnErr != nil {
			return nil, spawnErr
		}
		return fr, nil
	}}
}

func TestProvider_Complete_ComposesPromptAndReturnsContent(t *testing.T) {
	fr := &fakeRunner{reply: "salida del agente"}
	p := providerWith(fr, nil)

	resp, err := p.Complete(context.Background(), llm.CompletionOptions{
		Model:        "opencode",
		SystemPrompt: "sos un juez",
		Messages:     []llm.Message{{Role: "user", Content: "evaluá esto"}},
	})
	require.NoError(t, err)
	require.Equal(t, "salida del agente", resp.Content)
	require.Equal(t, "opencode", resp.Model)
	require.Equal(t, "stop", resp.FinishReason)
	require.Contains(t, fr.got, "sos un juez")
	require.Contains(t, fr.got, "evaluá esto")
	require.True(t, fr.closed, "Complete debe liberar el runner")
}

func TestProvider_Complete_SpawnError_Propagates(t *testing.T) {
	p := providerWith(nil, errors.New("no opencode"))
	_, err := p.Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
}

func TestProvider_Complete_PromptError_Propagates(t *testing.T) {
	fr := &fakeRunner{err: errors.New("boom")}
	p := providerWith(fr, nil)
	_, err := p.Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
	require.True(t, fr.closed, "aun con error el runner se cierra")
}

func TestProvider_Name_ReturnsACP(t *testing.T) {
	require.Equal(t, "acp", (&Provider{name: ProviderName}).Name())
}

func TestRegister_DefaultOn_RegistersProvider(t *testing.T) {
	f := llm.NewFactory()
	ok := Register(f, nil, acpbridge.Config{}, nil)
	require.True(t, ok)
	p, err := f.Get(ProviderName)
	require.NoError(t, err)
	require.Equal(t, ProviderName, p.Name())
}

func TestRegister_Disabled_SkipsRegistration(t *testing.T) {
	t.Setenv(DisabledEnv, "1")
	f := llm.NewFactory()
	require.False(t, Register(f, nil, acpbridge.Config{}, nil))
	_, err := f.Get(ProviderName)
	require.Error(t, err)
}
