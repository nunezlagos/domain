package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

// streamingProvider emite chunks via CompleteStream. El caller define
// el script: cada entry es (delta, isError, isDone). Si isError, el
// chunk lleva mensaje de error y el breaker debe registrar failure.
type streamingProvider struct {
	chunks []streamChunkSpec
}

type streamChunkSpec struct {
	delta   string
	isError bool
	isDone  bool
}

func (s *streamingProvider) Name() string { return "streaming" }
func (s *streamingProvider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	return nil, errors.New("Complete not supported in this test provider")
}
func (s *streamingProvider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, len(s.chunks))
	for _, c := range s.chunks {
		chunk := llm.StreamChunk{
			Delta: c.delta,
			Done:  c.isDone,
		}
		if c.isError {
			chunk.Error = "mid-stream failure"
		}
		ch <- chunk
	}
	close(ch)
	return ch, nil
}

// ISSUE-28.6: stream que termina con error mid-flight debe abrir el
// breaker (no solo errores del handshake inicial).
func TestBreaker_StreamErrorRecordsFailure(t *testing.T) {



	p := &streamingProvider{chunks: []streamChunkSpec{
		{delta: "hola", isDone: false},
		{delta: "mundo", isDone: false},
		{delta: "", isError: true, isDone: true},
	}}
	b := New(p, Config{FailureThreshold: 3, RecoveryTimeout: 100 * time.Millisecond})

	out, err := b.CompleteStream(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)


	for range out {
	}



	p2 := &streamingProvider{chunks: []streamChunkSpec{
		{delta: "x", isError: true, isDone: true},
	}}
	b2 := New(p2, Config{FailureThreshold: 3, RecoveryTimeout: 100 * time.Millisecond})
	for i := 0; i < 2; i++ {
		out2, _ := b2.CompleteStream(context.Background(), llm.CompletionOptions{})
		for range out2 {
		}
	}
	require.Equal(t, StateOpen, b2.State(),
		"3 streams con error mid-flight → Open (post-fix)")
}

// ISSUE-28.6: stream exitoso no debe registrar failure.
func TestBreaker_StreamSuccessRecordsSuccess(t *testing.T) {
	p := &streamingProvider{chunks: []streamChunkSpec{
		{delta: "hola", isDone: false},
		{delta: " mundo", isDone: true},
	}}
	b := New(p, Config{FailureThreshold: 3, RecoveryTimeout: 100 * time.Millisecond})

	out, err := b.CompleteStream(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)
	for range out {
	}

	require.Equal(t, StateClosed, b.State(), "stream exitoso no abre el breaker")
}

// ISSUE-28.6 escenario 3 (sabotaje): test que verifica que un error
// mid-stream deliberado cuenta para abrir el breaker. Si alguien rompe
// el fix (vuelve a `_ = sawError` o no llama recordFailure), este test
// falla.
func TestSabotage_StreamError_DeliberatelyOpensBreaker(t *testing.T) {



	p := &streamingProvider{chunks: []streamChunkSpec{
		{delta: "", isError: true, isDone: true},
	}}
	b := New(p, Config{FailureThreshold: 3, RecoveryTimeout: 100 * time.Millisecond})

	for i := 0; i < 5; i++ {
		out, _ := b.CompleteStream(context.Background(), llm.CompletionOptions{})
		for range out {
		}
	}

	require.Equal(t, StateOpen, b.State(),
		"sabotaje: 5 stream-errors con threshold=3 → breaker debe estar Open")
}
