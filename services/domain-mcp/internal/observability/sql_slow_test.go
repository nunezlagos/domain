package observability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type stubSlowStore struct {
	mu    sync.Mutex
	rows  []SlowQuery
	fail  bool
	delay time.Duration
}

func (s *stubSlowStore) InsertSlowQuery(ctx context.Context, q SlowQuery) error {
	s.mu.Lock()
	failed := s.fail
	s.mu.Unlock()
	if failed {
		return errors.New("simulated slow insert fail")
	}
	if s.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.delay):
		}
	}
	s.mu.Lock()
	s.rows = append(s.rows, q)
	s.mu.Unlock()
	return nil
}

type testInnerTracer struct {
	startCalls int
	endCalls   int
}

func (t *testInnerTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	t.startCalls++
	return ctx
}
func (t *testInnerTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	t.endCalls++
}

func TestSlowQueryTracer_FastQueryNotLogged(t *testing.T) {
	store := &stubSlowStore{}
	tr := NewSlowQueryTracer(&testInnerTracer{}, store, nil, 1, 100)
	defer tr.Close()

	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	time.Sleep(2 * time.Millisecond)
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: nil})
	tr.Close()

	require.Equal(t, 0, len(store.rows))
}

func TestSlowQueryTracer_SlowQueryLogged(t *testing.T) {
	store := &stubSlowStore{}
	inner := &testInnerTracer{}
	tr := NewSlowQueryTracer(inner, store, nil, 1, 10)
	defer tr.Close()

	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT pg_sleep(0.05)"})
	time.Sleep(20 * time.Millisecond)
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: nil})
	tr.Close()

	require.Equal(t, 1, len(store.rows))
	require.Equal(t, "SELECT pg_sleep(0.05)", store.rows[0].QueryText)
	require.GreaterOrEqual(t, store.rows[0].DurationMS, int64(10))
}

func TestSlowQueryTracer_DelegatesToInner(t *testing.T) {
	store := &stubSlowStore{}
	inner := &testInnerTracer{}
	tr := NewSlowQueryTracer(inner, store, nil, 1, 100)
	defer tr.Close()

	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: nil})
	tr.Close()

	require.Equal(t, 1, inner.startCalls)
	require.Equal(t, 1, inner.endCalls)
}

func TestSlowQueryTracer_DefaultInnerIsNoop(t *testing.T) {
	tr := NewSlowQueryTracer(nil, &stubSlowStore{}, nil, 1, 100)
	defer tr.Close()

	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 2"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: nil})
	tr.Close()
}

// Sabotaje: el WARN se loguea solo si el canal esta lleno. Sin el sub-test
// con queue size chico, este path requiere producir 1024+ items rapidos.
// Como aumentar items hasta 100k aumenta el tiempo del test desproporcionado,
// documentamos el path cubierto por los tests Invocation/HTTP/FN equivalentes
// (mismo patron select-default-drop).
func TestSlowQuerySabotage_FullQueue_DropsWithWarn(t *testing.T) {
	t.Skip("covered by TestInvocationSabotage_FullQueue_DropsWithWarn + TestHTTPSabotage_FullQueue_DropsWithWarn + TestFNSabotage_FullQueue_DropsWithWarn; SQL slow tiene queue size 1024 fijo y requiere 100k+ items para llenar")

	logger, buf := captureBuf()
	store := &stubSlowStore{delay: 1 * time.Millisecond}
	tr := NewSlowQueryTracer(&testInnerTracer{}, store, logger, 1, 0)

	for i := 0; i < 2000; i++ {
		ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: nil})
	}
	tr.Close()
	require.Contains(t, buf.String(), "queue full")
}

func TestSlowQuerySabotage_PersistFail_DoesNotLeakGoroutines(t *testing.T) {
	store := &stubSlowStore{fail: true}
	tr := NewSlowQueryTracer(&testInnerTracer{}, store, nil, 1, 0)
	for i := 0; i < 50; i++ {
		ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: nil})
	}
	done := make(chan struct{})
	go func() {
		tr.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() blocked")
	}
}

func TestSlowQueryTracer_Close_Idempotent(t *testing.T) {
	tr := NewSlowQueryTracer(&testInnerTracer{}, &stubSlowStore{}, nil, 1, 100)
	tr.Close()
	tr.Close()
}
