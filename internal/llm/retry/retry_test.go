package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

type fakeProvider struct {
	calls    atomic.Int32
	failures int32 // primeras N llamadas fallan
	err      error
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Complete(_ context.Context, _ llm.CompletionOptions) (*llm.Response, error) {
	n := f.calls.Add(1)
	if n <= f.failures {
		return nil, f.err
	}
	return &llm.Response{Content: "ok"}, nil
}

func (f *fakeProvider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	n := f.calls.Add(1)
	if n <= f.failures {
		return nil, f.err
	}
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func fastCfg() Config {
	return Config{MaxRetries: 3, BaseBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond}
}

func TestComplete_RetriesOn429(t *testing.T) {
	f := &fakeProvider{failures: 2, err: errors.New("anthropic 429: rate limit exceeded")}
	p := New(f, fastCfg())
	res, err := p.Complete(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)
	require.Equal(t, "ok", res.Content)
	require.Equal(t, int32(3), f.calls.Load(), "2 fallos + 1 éxito")
}

func TestComplete_NonTransient_NoRetry(t *testing.T) {
	f := &fakeProvider{failures: 99, err: errors.New("openai 401: invalid api key")}
	p := New(f, fastCfg())
	_, err := p.Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
	require.Equal(t, int32(1), f.calls.Load(), "auth error no debe reintentar")
}

func TestComplete_ExhaustsRetries(t *testing.T) {
	f := &fakeProvider{failures: 99, err: errors.New("google 503: overloaded")}
	p := New(f, fastCfg())
	_, err := p.Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
	require.Equal(t, int32(4), f.calls.Load(), "MaxRetries=3 → 4 intentos totales")
}

func TestCompleteStream_RetriesInitialError(t *testing.T) {
	f := &fakeProvider{failures: 1, err: errors.New("connection refused")}
	p := New(f, fastCfg())
	ch, err := p.CompleteStream(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Equal(t, int32(2), f.calls.Load())
}

func TestComplete_CtxCancelledDuringBackoff(t *testing.T) {
	f := &fakeProvider{failures: 99, err: errors.New("timeout")}
	p := New(f, Config{MaxRetries: 5, BaseBackoff: 200 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.Complete(ctx, llm.CompletionOptions{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestIsTransient_Matrix(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"anthropic 429: rate limited", true},
		{"openai 500: internal", true},
		{"google 503: overloaded", true},
		{"dial tcp: connection refused", true},
		{"context deadline exceeded (timeout)", true},
		{"openai 401: invalid api key", false},
		{"403 forbidden", false},
		{"400 bad request", false},
		{"model not found 404", false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, IsTransient(errors.New(tc.msg)), tc.msg)
	}
	require.False(t, IsTransient(nil))
}
