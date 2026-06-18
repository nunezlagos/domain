package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoverMiddleware_NoPanic_PassesThrough(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	h := RecoverMiddleware(nil)(next)
	req := httptest.NewRequest("GET", "/foo", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.True(t, called, "next handler debe ser llamado")
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "ok", rr.Body.String())
}

func TestRecoverMiddleware_Panic_Returns500(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom!")
	})

	h := RecoverMiddleware(nil)(next)
	req := httptest.NewRequest("GET", "/foo", nil)
	rr := httptest.NewRecorder()

	// No debe propagar el panic (si llega acá sin recover, falla el test).
	require.NotPanics(t, func() {
		h.ServeHTTP(rr, req)
	})

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Contains(t, rr.Body.String(), "internal server error")
}

func TestRecoverMiddleware_PanicAfterPartialWrite_DoesNotCrash(t *testing.T) {
	// Si el handler empieza a escribir y después panica, no podemos
	// cambiar el status code. El middleware debe swallow el panic
	// sin crashear el server. El cliente verá el response parcial
	// (con status code 200 que ya fue escrito) + connection drop.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		panic("after write")
	})

	h := RecoverMiddleware(nil)(next)
	req := httptest.NewRequest("GET", "/foo", nil)
	rr := httptest.NewRecorder()

	require.NotPanics(t, func() {
		h.ServeHTTP(rr, req)
	})

	// Status ya escrito por el handler antes del panic.
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "partial", rr.Body.String())
}

func TestRecoverMiddleware_WithLogger_LogsStack(t *testing.T) {
	var logBuf strings.Builder
	logger := newTestLogger(&logBuf)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("logged boom")
	})

	h := RecoverMiddleware(logger)(next)
	req := httptest.NewRequest("POST", "/api/v1/foo", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	logs := logBuf.String()
	require.Contains(t, logs, "PANIC recovered in HTTP handler")
	require.Contains(t, logs, "logged boom")
	require.Contains(t, logs, "/api/v1/foo")
	require.Contains(t, logs, "POST")
	require.Contains(t, logs, "stack")
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestRecoverMiddleware_NilLogger_DoesNotPanic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("nil logger safe")
	})

	h := RecoverMiddleware(nil)(next)
	req := httptest.NewRequest("GET", "/foo", nil)
	rr := httptest.NewRecorder()
	require.NotPanics(t, func() {
		h.ServeHTTP(rr, req)
	})
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}
