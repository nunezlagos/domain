package budget

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/tokens"
)

type fakeProvider struct {
	usagePerCall int
	chunks       []string
	calls        int
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Complete(_ context.Context, _ llm.CompletionOptions) (*llm.Response, error) {
	f.calls++
	return &llm.Response{Content: "ok", Usage: llm.Usage{TotalTokens: f.usagePerCall}}, nil
}

func (f *fakeProvider) CompleteStream(_ context.Context, _ llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	f.calls++
	ch := make(chan llm.StreamChunk, len(f.chunks)+1)
	for _, c := range f.chunks {
		ch <- llm.StreamChunk{Delta: c}
	}
	ch <- llm.StreamChunk{Done: true, Usage: &llm.Usage{}}
	close(ch)
	return ch, nil
}

func TestComplete_BlocksWhenExhausted(t *testing.T) {
	mgr, err := tokens.NewTokenBudget(0, 100, 0, tokens.ModeError)
	require.NoError(t, err)
	f := &fakeProvider{usagePerCall: 80}
	p := New(f, mgr)


	_, err = p.Complete(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)

	_, err = p.Complete(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)

	_, err = p.Complete(context.Background(), llm.CompletionOptions{})
	require.ErrorIs(t, err, tokens.ErrBudgetExceeded)
	require.Equal(t, 2, f.calls, "el provider no se llama con budget agotado")
}

func TestCompleteStream_TruncatesGracefully(t *testing.T) {

	mgr, err := tokens.NewTokenBudget(0, 5, 0, tokens.ModeTruncate)
	require.NoError(t, err)
	long := strings.Repeat("palabra ", 20) // >>5 tokens estimados
	f := &fakeProvider{chunks: []string{long, long, long}}
	p := New(f, mgr)

	ch, err := p.CompleteStream(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)

	var deltas int
	var done bool
	for chunk := range ch {
		if chunk.Done {
			done = true
			continue
		}
		deltas++
	}
	require.True(t, done, "truncate corta con Done graceful")
	require.Less(t, deltas, 3, "no deben llegar todos los chunks")
	require.True(t, mgr.State().Truncated)
}

func TestNew_NilManagerPassthrough(t *testing.T) {
	f := &fakeProvider{usagePerCall: 1}
	p := New(f, nil)
	require.Equal(t, f, p, "mgr nil → provider sin envolver")
}
