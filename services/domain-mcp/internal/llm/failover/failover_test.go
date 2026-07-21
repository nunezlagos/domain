package failover

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/circuitbreaker"
)

type fakeProv struct {
	name  string
	err   error
	resp  string
	calls *int
}

func (f fakeProv) Name() string { return f.name }

func (f fakeProv) Complete(context.Context, llm.CompletionOptions) (*llm.Response, error) {
	if f.calls != nil {
		*f.calls++
	}
	if f.err != nil {
		return nil, f.err
	}
	return &llm.Response{Content: f.resp}, nil
}

func (f fakeProv) CompleteStream(context.Context, llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Delta: f.resp, Done: true}
	close(ch)
	return ch, nil
}

func TestFailover_PrimaryTransient_SecondaryServes(t *testing.T) {
	var served []string
	obs := func(p, r string) { served = append(served, p+":"+r) }
	chain := []llm.Provider{
		fakeProv{name: "minimax", err: errors.New("minimax 429: rate limit exceeded")},
		fakeProv{name: "openai", resp: "ok"},
	}
	fp := New("rerank-failover", chain, obs)
	resp, err := fp.Complete(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Contains(t, served, "openai:success")
}

func TestFailover_FatalError_StopsChain(t *testing.T) {
	secondaryCalls := 0
	chain := []llm.Provider{
		fakeProv{name: "anthropic", err: errors.New("anthropic 401: invalid api key")},
		fakeProv{name: "openai", resp: "ok", calls: &secondaryCalls},
	}
	fp := New("f", chain, nil)
	_, err := fp.Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid api key")
	require.Equal(t, 0, secondaryCalls, "un error fatal (auth) no debe gastar la cadena")
}

func TestFailover_CircuitOpen_SkipsToNextProvider(t *testing.T) {
	chain := []llm.Provider{
		fakeProv{name: "minimax", err: circuitbreaker.ErrCircuitOpen},
		fakeProv{name: "openai", resp: "ok"},
	}
	fp := New("f", chain, nil)
	resp, err := fp.Complete(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
}

func TestFailover_AllTransientFail_ReturnsLastError(t *testing.T) {
	chain := []llm.Provider{
		fakeProv{name: "minimax", err: errors.New("minimax 429: rate limit")},
		fakeProv{name: "openai", err: errors.New("openai 503: overloaded")},
	}
	fp := New("f", chain, nil)
	_, err := fp.Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "503")
}

func TestForRole_NoChainEnv_ReturnsSingleProviderWithRoleModel(t *testing.T) {
	f := llm.NewFactory()
	f.Register("minimax", fakeProv{name: "minimax", resp: "ok"})
	p, model, err := ForRole(f, llm.RoleRerank, nil)
	require.NoError(t, err)
	require.Equal(t, "minimax", p.Name())
	require.Equal(t, "MiniMax-M3", model)
}

func TestForRole_ChainEnv_BuildsFailoverChain(t *testing.T) {
	t.Setenv("DOMAIN_LLM_RERANK_CHAIN", "minimax,openai")
	f := llm.NewFactory()
	f.Register("minimax", fakeProv{name: "minimax", err: errors.New("minimax 429: rate limit")})
	f.Register("openai", fakeProv{name: "openai", resp: "ok"})
	p, _, err := ForRole(f, llm.RoleRerank, nil)
	require.NoError(t, err)
	resp, err := p.Complete(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
}
