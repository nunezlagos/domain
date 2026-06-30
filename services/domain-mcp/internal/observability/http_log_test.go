package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubHTTPLogStore struct {
	mu     sync.Mutex
	logs   []HTTPLog
	fail   bool
	insert time.Duration
}

func (s *stubHTTPLogStore) InsertHTTPLog(ctx context.Context, l HTTPLog) error {
	s.mu.Lock()
	failed := s.fail
	s.mu.Unlock()
	if failed {
		return errors.New("simulated insert fail")
	}
	if s.insert > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.insert):
		}
	}
	s.mu.Lock()
	s.logs = append(s.logs, l)
	s.mu.Unlock()
	return nil
}

func (s *stubHTTPLogStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.logs)
}

func captureBuf() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})), buf
}

func nextHandler(captured *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Add(1)
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("body-out"))
	})
}

func TestHTTPLogger_Middleware_WritesRow(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)
	defer h.Close()

	var hits atomic.Int32
	mw := h.Middleware(nextHandler(&hits))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("User-Agent", "test/1.0")
	req.ContentLength = 42
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	h.Close()
	require.EqualValues(t, 1, hits.Load())
	require.Equal(t, 1, store.count())
	require.Equal(t, "/health", store.logs[0].Path)
	require.Equal(t, http.StatusTeapot, store.logs[0].Status)
	require.Equal(t, "test/1.0", store.logs[0].UserAgent)
	require.Equal(t, 42, store.logs[0].BytesIn)
	require.Equal(t, len("body-out"), store.logs[0].BytesOut)
	require.NotEqual(t, uuid.Nil, store.logs[0].RequestID)
}

func TestHTTPLogger_Middleware_SetsRequestIDHeader(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)
	defer h.Close()

	mw := h.Middleware(nextHandler(&atomic.Int32{}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	mw.ServeHTTP(rec, req)

	got := rec.Header().Get("X-Request-Id")
	_, err := uuid.Parse(got)
	require.NoError(t, err)
}

func TestHTTPLogger_RequestIDFromContext(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)
	defer h.Close()

	_, buf := captureBuf()
	_ = buf

	var seenID uuid.UUID
	mw := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenID = RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	mw.ServeHTTP(rec, req)

	require.NotEqual(t, uuid.Nil, seenID)
}

func TestHTTPLogger_RequestIDFromContext_Empty(t *testing.T) {
	require.Equal(t, uuid.Nil, RequestIDFromContext(context.Background()))
}

func TestHTTPLogger_Close_Idempotent(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)
	h.Close()
	h.Close()
}

func TestSabotage_FullQueue_DropsWithWarn(t *testing.T) {
	logger, buf := captureBuf()
	h := NewHTTPLogger(&stubHTTPLogStore{insert: 100 * time.Millisecond}, logger, 1)
	mw := h.Middleware(nextHandler(&atomic.Int32{}))

	for i := 0; i < 2000; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		mw.ServeHTTP(httptest.NewRecorder(), req)
	}
	h.Close()
	require.Contains(t, buf.String(), "queue full")
}

func TestSabotage_StoreFail_DoesNotLeakGoroutines(t *testing.T) {
	store := &stubHTTPLogStore{fail: true}
	logger, _ := captureBuf()
	h := NewHTTPLogger(store, logger, 2)

	mw := h.Middleware(nextHandler(&atomic.Int32{}))
	for i := 0; i < 5; i++ {
		mw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	}
	done := make(chan struct{})
	go func() {
		h.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() blocked: persist failure leaked goroutines")
	}
}

func TestHTTPLogger_ConcurrentServedSafe(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 4)
	defer h.Close()

	var hits atomic.Int32
	mw := h.Middleware(nextHandler(&hits))
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			mw.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}
	wg.Wait()
	h.Close()

	require.EqualValues(t, 50, hits.Load())
	require.Equal(t, 50, store.count())
}

func TestSabotage_DefaultLevelsInWarn(t *testing.T) {
	logger, buf := captureBuf()
	store := &stubHTTPLogStore{fail: true}
	h := NewHTTPLogger(store, logger, 1)
	mw := h.Middleware(nextHandler(&atomic.Int32{}))
	mw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/fail", nil))
	h.Close()
	require.True(t, strings.Contains(buf.String(), "persist failed"),
		"expected 'persist failed' in log, got: %s", buf.String())
}
