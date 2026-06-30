package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

type slowProvider struct {
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	delay    time.Duration
}

func (s *slowProvider) Name() string { return "slow" }

func (s *slowProvider) Complete(ctx context.Context, _ llm.CompletionOptions) (*llm.Response, error) {
	cur := s.inFlight.Add(1)
	for {
		max := s.maxSeen.Load()
		if cur <= max || s.maxSeen.CompareAndSwap(max, cur) {
			break
		}
	}
	defer s.inFlight.Add(-1)
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &llm.Response{Content: "ok"}, nil
}

func (s *slowProvider) CompleteStream(ctx context.Context, _ llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	go func() {
		time.Sleep(s.delay)
		ch <- llm.StreamChunk{Done: true}
		close(ch)
	}()
	return ch, nil
}

// El semáforo acota la concurrencia real vista por el provider.
func TestComplete_BoundsConcurrency(t *testing.T) {
	slow := &slowProvider{delay: 50 * time.Millisecond}
	p := New(slow, 2)

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Complete(context.Background(), llm.CompletionOptions{})
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	require.LessOrEqual(t, slow.maxSeen.Load(), int32(2),
		"el provider nunca debe ver más de maxConcurrent llamadas simultáneas")
}

// Waiter aborta limpio si su ctx muere antes de obtener slot.
func TestComplete_CtxCancelWhileWaiting(t *testing.T) {
	slow := &slowProvider{delay: 300 * time.Millisecond}
	p := New(slow, 1)

	go p.Complete(context.Background(), llm.CompletionOptions{}) // ocupa el slot
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.Complete(ctx, llm.CompletionOptions{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// El slot del stream se libera recién cuando el canal cierra.
func TestCompleteStream_HoldsSlotUntilDone(t *testing.T) {
	slow := &slowProvider{delay: 100 * time.Millisecond}
	p := New(slow, 1)

	ch, err := p.CompleteStream(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)


	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = p.Complete(ctx, llm.CompletionOptions{})
	require.ErrorIs(t, err, context.DeadlineExceeded)

	for range ch {
	} // drenar → libera slot

	_, err = p.Complete(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err, "slot liberado tras cerrar el stream")
}

func TestNew_DefaultConcurrency(t *testing.T) {
	p := New(&slowProvider{}, 0).(*provider)
	require.Equal(t, 8, cap(p.sem))
}
