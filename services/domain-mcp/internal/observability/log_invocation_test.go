package observability

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func captureLogger() (*slog.Logger, *threadSafeBuffer) {
	tb := &threadSafeBuffer{}
	h := slog.NewJSONHandler(tb, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), tb
}

type stubStore struct {
	mu       sync.Mutex
	calls    []Invocation
	failNext bool
	insertTO time.Duration
}

func (s *stubStore) InsertInvocation(ctx context.Context, inv Invocation) error {
	s.mu.Lock()
	s.calls = append(s.calls, inv)
	fail := s.failNext
	s.mu.Unlock()
	if fail {
		return errors.New("simulated db error")
	}
	if s.insertTO > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.insertTO):
		}
	}
	return nil
}

func TestInvocationLogger_Log_Enqueues(t *testing.T) {
	store := &stubStore{}
	l := NewInvocationLogger(store, nil, 1, 16)
	l.Log(Invocation{ToolName: "x", Status: "ok", DurationMS: 5})
	l.Close()
	require.Equal(t, 1, len(store.calls))
}

func TestInvocationLogger_Log_NonBlockingOnFullQueue(t *testing.T) {
	store := &stubStore{insertTO: 50 * time.Millisecond}
	l := NewInvocationLogger(store, nil, 1, 2)

	for i := 0; i < 100; i++ {
		l.Log(Invocation{ToolName: "x", Status: "ok", DurationMS: 1})
	}
	l.Close()

	// No esperamos 100 inserts; aceptamos >= 1 pero <= queueCap+workers.
	require.GreaterOrEqual(t, len(store.calls), 1)
	require.LessOrEqual(t, len(store.calls), 4)
}

func TestInvocationLogger_HashArgs(t *testing.T) {
	require.Nil(t, HashArgs(nil))
	require.Nil(t, HashArgs([]byte{}))
	a := HashArgs([]byte("hello"))
	b := HashArgs([]byte("hello"))
	c := HashArgs([]byte("world"))
	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
	require.Len(t, a, 32)
}

func TestInvocationLogger_Close_Idempotent(t *testing.T) {
	l := NewInvocationLogger(&stubStore{}, nil, 1, 4)
	l.Close()
	require.NotPanics(t, func() { l.Close() })
}

func TestInvocationLogger_Persist_NilUUIDsNoPanic(t *testing.T) {
	store := &stubStore{}
	l := NewInvocationLogger(store, nil, 1, 4)
	l.Log(Invocation{
		ToolName:    "no_principal",
		Status:      "ok",
		DurationMS:  7,
		PrincipalID: uuid.Nil,
		OrgID:       uuid.Nil,
		ProjectID:   uuid.Nil,
	})
	l.Close()
	require.Equal(t, 1, len(store.calls))
	require.Equal(t, uuid.Nil, store.calls[0].PrincipalID)
}

func TestInvocationLogger_Persist_DropsOnError_NoPanic(t *testing.T) {
	store := &stubStore{failNext: true}
	logger, buf := captureLogger()
	l := NewInvocationLogger(store, logger, 1, 4)
	l.Log(Invocation{ToolName: "broken", Status: "ok"})
	l.Close()
	require.Contains(t, buf.String(), "invocation persist failed")
}

func TestInvocationLogger_Log_ConcurrentSafe(t *testing.T) {
	store := &stubStore{}
	l := NewInvocationLogger(store, nil, 2, 1024)

	var wg sync.WaitGroup
	var counter atomic.Int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Log(Invocation{ToolName: "conc", Status: "ok"})
			counter.Add(1)
		}()
	}
	wg.Wait()
	l.Close()
	require.EqualValues(t, 100, counter.Load())
}

func TestInvocationSabotage_FullQueue_DropsWithWarn(t *testing.T) {
	logger, buf := captureLogger()
	store := &stubStore{insertTO: 100 * time.Millisecond}
	l := NewInvocationLogger(store, logger, 1, 1)

	for i := 0; i < 50; i++ {
		l.Log(Invocation{ToolName: "overflow", Status: "error"})
	}
	l.Close()

	sawWarn := false
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "queue full") {
			sawWarn = true
			break
		}
	}
	require.True(t, sawWarn, "expected WARN 'queue full' when buffer overflows")
}

func TestInvocationSabotage_StoreError_DoesNotLeakGoroutines(t *testing.T) {
	store := &stubStore{failNext: true, insertTO: 50 * time.Millisecond}
	logger, _ := captureLogger()
	l := NewInvocationLogger(store, logger, 2, 4)

	for i := 0; i < 10; i++ {
		l.Log(Invocation{ToolName: "leak", Status: "ok"})
	}

	done := make(chan struct{})
	go func() {
		l.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close() blocked: persist failure leaked goroutine state")
	}
}

func TestInvocationSabotage_BreakingPersist_PushesWarnNotPanic(t *testing.T) {
	store := &stubStore{failNext: true}
	logger, buf := captureLogger()
	l := NewInvocationLogger(store, logger, 1, 4)
	l.Log(Invocation{ToolName: "broken_invocation_test", Status: "ok"})
	l.Close()
	require.NotContains(t, buf.String(), "panic:")
	require.Contains(t, buf.String(), "invocation persist failed")
}
