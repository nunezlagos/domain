package circuitbreaker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

type flakyProvider struct {
	failTimes int32
	calls     int32
}

func (f *flakyProvider) Name() string { return "flaky" }
func (f *flakyProvider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	atomic.AddInt32(&f.calls, 1)
	if atomic.LoadInt32(&f.failTimes) > 0 {
		atomic.AddInt32(&f.failTimes, -1)
		return nil, errors.New("provider down")
	}
	return &llm.Response{Content: "ok"}, nil
}
func (f *flakyProvider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

func TestBreaker_StaysClosedOnSuccess(t *testing.T) {
	b := New(&flakyProvider{}, Config{FailureThreshold: 3})
	for i := 0; i < 5; i++ {
		_, err := b.Complete(context.Background(), llm.CompletionOptions{})
		require.NoError(t, err)
	}
	require.Equal(t, StateClosed, b.State())
}

func TestBreaker_OpensAfterThreshold(t *testing.T) {
	fp := &flakyProvider{failTimes: 100}
	b := New(fp, Config{FailureThreshold: 3, RecoveryTimeout: 100 * time.Millisecond})
	for i := 0; i < 3; i++ {
		_, _ = b.Complete(context.Background(), llm.CompletionOptions{})
	}
	require.Equal(t, StateOpen, b.State(), "después de 3 fails → Open")

	// 4to call NO debe llegar al provider
	prevCalls := atomic.LoadInt32(&fp.calls)
	_, err := b.Complete(context.Background(), llm.CompletionOptions{})
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.Equal(t, prevCalls, atomic.LoadInt32(&fp.calls), "no debe invocar provider mientras Open")
}

func TestBreaker_RecoversThroughHalfOpen(t *testing.T) {
	fp := &flakyProvider{failTimes: 3}
	b := New(fp, Config{FailureThreshold: 3, RecoveryTimeout: 50 * time.Millisecond})
	for i := 0; i < 3; i++ {
		_, _ = b.Complete(context.Background(), llm.CompletionOptions{})
	}
	require.Equal(t, StateOpen, b.State())

	time.Sleep(60 * time.Millisecond) // espera recovery timeout

	// HalfOpen probe: el flakyProvider ya no falla (failTimes = 0)
	_, err := b.Complete(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)
	require.Equal(t, StateClosed, b.State(), "success en HalfOpen → Closed")
}

func TestBreaker_HalfOpenFailureReopens(t *testing.T) {
	fp := &flakyProvider{failTimes: 4} // 3 fails para abrir + 1 más en HalfOpen
	b := New(fp, Config{FailureThreshold: 3, RecoveryTimeout: 30 * time.Millisecond})
	for i := 0; i < 3; i++ {
		_, _ = b.Complete(context.Background(), llm.CompletionOptions{})
	}
	require.Equal(t, StateOpen, b.State())
	time.Sleep(40 * time.Millisecond)

	// HalfOpen probe: provider sigue fallando
	_, err := b.Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
	require.Equal(t, StateOpen, b.State(), "fail en HalfOpen → reabre")
}

// Sabotaje: success aislado en medio del threshold NO debe cerrar el breaker
// (este patrón evita falsos positivos)
func TestSabotage_Breaker_NeedsConsecutiveFails(t *testing.T) {
	// 2 fails → 1 ok → 2 fails: cada success resetea el counter
	type step struct{ ok bool }
	provider := &scriptedProvider{}
	provider.script = []bool{false, false, true, false, false, true}
	b := New(provider, Config{FailureThreshold: 3})
	for i := 0; i < 6; i++ {
		_, _ = b.Complete(context.Background(), llm.CompletionOptions{})
	}
	require.Equal(t, StateClosed, b.State(),
		"intermitencias con success entre fails no deben abrir el breaker")
}

type scriptedProvider struct {
	script []bool // true=ok, false=fail
	idx    int
}

func (s *scriptedProvider) Name() string { return "scripted" }
func (s *scriptedProvider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	if s.idx >= len(s.script) {
		return &llm.Response{Content: "ok"}, nil
	}
	ok := s.script[s.idx]
	s.idx++
	if !ok {
		return nil, errors.New("fail")
	}
	return &llm.Response{Content: "ok"}, nil
}
func (s *scriptedProvider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	return nil, nil
}
