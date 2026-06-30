package observability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubFnStore struct {
	mu   sync.Mutex
	rows []FnLogEntry
	fail bool
}

func (s *stubFnStore) InsertFnLog(ctx context.Context, e FnLogEntry) error {
	s.mu.Lock()
	failed := s.fail
	s.mu.Unlock()
	if failed {
		return errors.New("simulated fn insert fail")
	}
	s.mu.Lock()
	s.rows = append(s.rows, e)
	s.mu.Unlock()
	return nil
}

func TestFnLogger_Enter_RecordsOnExit(t *testing.T) {
	store := &stubFnStore{}
	f := NewFnLogger(store, nil, 1)
	defer f.Close()

	exit := f.Enter("observation.Save", "observation", []byte(`{"k":1}`))
	time.Sleep(2 * time.Millisecond)
	exit(nil)
	f.Close()

	require.Equal(t, 1, len(store.rows))
	require.Equal(t, "observation.Save", store.rows[0].FnName)
	require.Equal(t, "observation", store.rows[0].Pkg)
	require.Equal(t, "ok", store.rows[0].Status)
	require.GreaterOrEqual(t, store.rows[0].DurationUS, int64(0))
	require.NotNil(t, store.rows[0].ArgsHash)
}

func TestFnLogger_Enter_CapturesError(t *testing.T) {
	store := &stubFnStore{}
	f := NewFnLogger(store, nil, 1)
	defer f.Close()

	exit := f.Enter("observation.Save", "observation", nil)
	exit(errors.New("validation failed: foo"))
	f.Close()

	require.Equal(t, 1, len(store.rows))
	require.Equal(t, "error", store.rows[0].Status)
	require.Equal(t, "validation failed: foo", store.rows[0].ErrorMessage)
}

func TestFnLogger_Trace_RecoversPanic(t *testing.T) {
	store := &stubFnStore{}
	f := NewFnLogger(store, nil, 1)
	defer f.Close()

	gotErr := f.Trace("dangerous.fn", "dangerous", nil, func() error {
		panic("boom")
	})
	f.Close()

	require.Error(t, gotErr)
	require.Equal(t, 1, len(store.rows))
	require.Equal(t, "panic: boom", store.rows[0].ErrorMessage)
}

func TestFnLogger_Trace_PassesThroughOnError(t *testing.T) {
	store := &stubFnStore{}
	f := NewFnLogger(store, nil, 1)
	defer f.Close()

	want := errors.New("real error")
	got := f.Trace("fn", "pkg", nil, func() error { return want })
	f.Close()

	require.ErrorIs(t, got, want)
	require.Equal(t, "error", store.rows[0].Status)
}

func TestFnLogger_Trace_NoErrorReturnsNil(t *testing.T) {
	store := &stubFnStore{}
	f := NewFnLogger(store, nil, 1)
	defer f.Close()

	got := f.Trace("fn", "pkg", nil, func() error { return nil })
	f.Close()

	require.NoError(t, got)
	require.Equal(t, "ok", store.rows[0].Status)
}

func TestSabotage_FullQueue_DropsWithWarn(t *testing.T) {
	logger, buf := captureBuf()
	store := &stubFnStore{}
	f := NewFnLogger(store, logger, 1)

	for i := 0; i < 2000; i++ {
		// encolar muchas invocaciones sin consumir
		go f.Enter("bulk", "pkg", nil)(nil)
	}
	f.Close()
	require.Contains(t, buf.String(), "queue full")
}

func TestSabotage_StoreFail_DoesNotLeakGoroutines(t *testing.T) {
	store := &stubFnStore{fail: true}
	f := NewFnLogger(store, nil, 1)

	for i := 0; i < 50; i++ {
		f.Enter("bulk", "pkg", nil)(nil)
	}
	done := make(chan struct{})
	go func() {
		f.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() blocked")
	}
}

func TestFnLogger_Close_Idempotent(t *testing.T) {
	store := &stubFnStore{}
	f := NewFnLogger(store, nil, 1)
	f.Close()
	f.Close()
}
