package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/apikey"
)

// issue-13.4 — Tests de comportamiento del Idempotency middleware.
// Reglas testeadas:
//  1. GET/HEAD/OPTIONS ignoran Idempotency-Key
//  2. POST/PATCH/DELETE sin key: handler corre normal
//  3. POST sin principal: handler corre normal (skip)
//  4. Misma key + mismo body hash: replay cached
//  5. Misma key + body distinto: 409 Conflict
//  6. 5xx NO se cachea; 2xx/4xx SI
//  7. Header Idempotent-Replayed: true SOLO en replay
//  8. Scoping por org: lookup recibe orgID del Principal
//  9. Lookup error != ErrNoRows: degrada gracefully

// --- fakePool: DBPool mock ---
type fakePool struct {
	mu          sync.Mutex
	lookupFn    func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn      func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	lookupCalls int
	execCalls   int
}

func (f *fakePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	f.mu.Lock()
	f.lookupCalls++
	f.mu.Unlock()
	if f.lookupFn != nil {
		return f.lookupFn(ctx, sql, args...)
	}
	return &fakeRow{err: pgx.ErrNoRows}
}

func (f *fakePool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.mu.Lock()
	f.execCalls++
	f.mu.Unlock()
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

type fakeRow struct {
	err  error
	scan func(dest ...any) error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.scan != nil {
		return r.scan(dest...)
	}
	return r.err
}

// testHandler cuenta invocaciones y retorna status+body fijos.
type testHandler struct {
	calls  int
	status int
	body   string
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.calls++
	w.WriteHeader(h.status)
	_, _ = io.WriteString(w, h.body)
}

func principalForTest() *apikey.Principal {
	return &apikey.Principal{
		OrganizationID: uuid.New().String(),
		UserID:         uuid.New().String(),
		APIKeyID:       uuid.New().String(),
		Role:           "owner",
	}
}

// makeReq crea un *http.Request con el Principal inyectado via apikey.WithPrincipal.
// Patron: usamos el helper publico del package apikey (no la key privada).
func makeReq(method, key, body string, p *apikey.Principal) *http.Request {
	req := httptest.NewRequest(method, "/test", bytes.NewBufferString(body))
	if key != "" {
		req.Header.Set(HeaderIdempotencyKey, key)
	}
	if p != nil {
		req = req.WithContext(apikey.WithPrincipal(req.Context(), p))
	}
	return req
}

// === Tests de comportamiento ===

// Comportamiento: GET ignora Idempotency-Key (no es mutation).
func TestBehavior_GET_IgnoresIdempotencyKey(t *testing.T) {
	pool := &fakePool{}
	h := &testHandler{status: 200, body: "ok"}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set(HeaderIdempotencyKey, "any-key")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, 1, h.calls, "GET DEBE invocar handler")
	require.Equal(t, 0, pool.lookupCalls, "GET NO debe tocar cache")
}

// Comportamiento: POST sin Idempotency-Key → handler corre normal (no forzar).
func TestBehavior_POST_NoKey_RunsNormally(t *testing.T) {
	pool := &fakePool{}
	h := &testHandler{status: 201, body: `{"id":"new"}`}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	p := principalForTest()
	req := makeReq("POST", "", `{"name":"x"}`, p)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, 1, h.calls)
	require.Equal(t, 201, rec.Code)
	require.Equal(t, 0, pool.lookupCalls, "sin key, no lookup")
}

// Comportamiento: POST sin Principal → handler corre normal (skip).
// Auth es responsabilidad del middleware previo.
func TestBehavior_POST_NoPrincipal_RunsNormally(t *testing.T) {
	pool := &fakePool{}
	h := &testHandler{status: 200, body: "ok"}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	req := makeReq("POST", "key-123", `{"x":1}`, nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, 1, h.calls, "sin principal, handler corre normal")
	require.Equal(t, 0, pool.lookupCalls, "sin principal, skip middleware")
}

// Comportamiento: misma key + mismo body → replay cached (handler NO corre).
func TestBehavior_SameKeySameBody_Replays(t *testing.T) {
	cachedBody := `{"id":"abc"}`
	storedHash := sha256.Sum256([]byte(`{"name":"x"}`))

	pool := &fakePool{
		lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{
				scan: func(dest ...any) error {
					*(dest[0].(*int16)) = int16(201)
					*(dest[1].(*[]byte)) = []byte(`{}`)
					*(dest[2].(*[]byte)) = []byte(cachedBody)
					*(dest[3].(*[]byte)) = storedHash[:]
					return nil
				},
			}
		},
	}

	h := &testHandler{status: 201, body: cachedBody}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	p := principalForTest()
	req := makeReq("POST", "key-replay", `{"name":"x"}`, p)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, 0, h.calls, "handler NO debe invocarse en replay")
	require.Equal(t, 201, rec.Code)
	require.Equal(t, cachedBody, rec.Body.String())
	require.Equal(t, "true", rec.Header().Get(HeaderReplayed))
}

// Comportamiento: misma key + body distinto → 409 Conflict (no silent overwrite).
func TestBehavior_SameKeyDifferentBody_Conflicts(t *testing.T) {
	originalHash := sha256.Sum256([]byte(`{"name":"ORIGINAL"}`))

	pool := &fakePool{
		lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{
				scan: func(dest ...any) error {
					*(dest[0].(*int16)) = int16(201)
					*(dest[1].(*[]byte)) = []byte(`{}`)
					*(dest[2].(*[]byte)) = []byte(`{"id":"1"}`)
					*(dest[3].(*[]byte)) = originalHash[:]
					return nil
				},
			}
		},
	}

	h := &testHandler{status: 201, body: `{"id":"new"}`}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	p := principalForTest()
	req := makeReq("POST", "key-conflict", `{"name":"DIFFERENT"}`, p)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.Equal(t, 0, h.calls, "handler NO debe invocarse en conflict")
	require.Contains(t, rec.Body.String(), "idempotency_mismatch")
}

// Comportamiento: 5xx NO se cachea (server errors son no-deterministicos).
func TestBehavior_5xxNotCached(t *testing.T) {
	pool := &fakePool{
		lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			t.Errorf("Exec NO debe llamarse para 5xx (sql: %s)", sql)
			return pgconn.CommandTag{}, nil
		},
	}

	h := &testHandler{status: 500, body: "internal error"}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	p := principalForTest()
	req := makeReq("POST", "key-5xx", `{"x":1}`, p)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, 500, rec.Code)
	require.Equal(t, 1, h.calls)
	require.Equal(t, 0, pool.execCalls, "5xx NO debe cachearse")
}

// Comportamiento: 2xx/4xx SI se cachean.
func TestBehavior_2xx4xx_Cached(t *testing.T) {
	for _, status := range []int{200, 201, 204, 400, 422, 409} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			pool := &fakePool{
				lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &fakeRow{err: pgx.ErrNoRows}
				},
			}

			h := &testHandler{status: status, body: "x"}
			mw := (&Idempotency{Pool: pool}).Wrap(h)

			p := principalForTest()
			req := makeReq("POST", "key-"+http.StatusText(status), `{}`, p)
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			require.Equal(t, status, rec.Code)
			require.Equal(t, 1, pool.execCalls, "Exec (cache) DEBE llamarse para status %d", status)
		})
	}
}

// Comportamiento: PATCH y DELETE también se benefician de idempotency.
func TestBehavior_PATCHAndDELETE_AlsoIdempotent(t *testing.T) {
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			require.True(t, shouldApply(httptest.NewRequest(method, "/x", nil)))
		})
	}
}

// Comportamiento: HEAD/OPTIONS no se benefician (no mutan estado).
func TestBehavior_HEADAndOPTIONS_NotIdempotent(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		t.Run(method, func(t *testing.T) {
			require.False(t, shouldApply(httptest.NewRequest(method, "/x", nil)))
		})
	}
}

// Comportamiento: header Idempotent-Replayed SOLO en replay.
func TestBehavior_FirstRequest_NoReplayHeader(t *testing.T) {
	pool := &fakePool{
		lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	h := &testHandler{status: 201, body: "x"}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	p := principalForTest()
	req := makeReq("POST", "key-first", `{}`, p)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Empty(t, rec.Header().Get(HeaderReplayed),
		"primera request NO debe marcarse como replayed")
	require.Equal(t, 1, h.calls)
}

// Comportamiento: body del request se pasa intacto al handler downstream
// (NopCloser restaura el body despues de leerlo para hash).
func TestBehavior_RequestBodyPassedToHandler(t *testing.T) {
	pool := &fakePool{
		lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	var received string
	customHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.WriteHeader(200)
	})
	mw := (&Idempotency{Pool: pool}).Wrap(customHandler)

	p := principalForTest()
	expected := `{"name":"preserved"}`
	req := makeReq("POST", "key-body", expected, p)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, expected, received,
		"body debe pasarse intacto al handler (NopCloser lo restaura)")
}

// Comportamiento: lookup recibe orgID del Principal (scoping entre orgs).
// Keys de org A NO deben cache-hit requests de org B con misma key.
func TestBehavior_OrgIDScope_PassedToLookup(t *testing.T) {
	p := principalForTest()
	expectedOrg, _ := uuid.Parse(p.OrganizationID)
	expectedKey := "key-org-scope"

	var capturedArgs []any
	pool := &fakePool{
		lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedArgs = args
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}

	h := &testHandler{status: 200, body: "x"}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	req := makeReq("POST", expectedKey, `{}`, p)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, 1, pool.lookupCalls)
	require.Len(t, capturedArgs, 2, "lookup recibe 2 args: orgID, key")
	capturedOrg, ok := capturedArgs[0].(uuid.UUID)
	require.True(t, ok, "arg[0] debe ser uuid.UUID")
	require.Equal(t, expectedOrg, capturedOrg,
		"OrgID del Principal debe pasarse al lookup (scoping entre orgs)")
	require.Equal(t, expectedKey, capturedArgs[1])
}

// Sabotaje: DB lookup falla con error != ErrNoRows → handler corre normal.
// Defense in depth: no romper el request por falla de cache.
func TestSabotage_LookupError_FallsThrough(t *testing.T) {
	pool := &fakePool{
		lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: errors.New("connection lost")}
		},
	}
	h := &testHandler{status: 200, body: "x"}
	mw := (&Idempotency{Pool: pool}).Wrap(h)

	p := principalForTest()
	req := makeReq("POST", "key-err", `{}`, p)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, 1, h.calls, "handler corre cuando lookup falla con error")
	require.Equal(t, 200, rec.Code)
}

// Sanity: shouldApply exportado se puede usar en tests de integracion.
func TestShouldApply_Coverage(t *testing.T) {
	methods := []struct {
		method string
		want   bool
	}{
		{"GET", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"POST", true},
		{"PUT", false},     // PUT no esta en la lista (decision de diseno)
		{"PATCH", true},
		{"DELETE", true},
		{"", false},
	}
	for _, tc := range methods {
		t.Run(tc.method, func(t *testing.T) {
			got := shouldApply(httptest.NewRequest(tc.method, "/x", nil))
			require.Equal(t, tc.want, got)
		})
	}
}

// Sabotaje final: asegurar que el ctx no se rompe con un body muy grande.
// El middleware debe leer el body completo (para hash) y restaurarlo.
// Body de 1MB no debe causar OOM ni panico.
func TestSabotage_LargeBody(t *testing.T) {
	pool := &fakePool{
		lookupFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	var receivedLen int
	customHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedLen = len(body)
		w.WriteHeader(200)
	})
	mw := (&Idempotency{Pool: pool}).Wrap(customHandler)

	// Body de 512KB (menor que maxOutputBytes de sandbox = 1MB; suficiente)
	bigBody := bytes.Repeat([]byte("x"), 512*1024)
	p := principalForTest()
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(bigBody))
	req.Header.Set(HeaderIdempotencyKey, "key-big")
	req = req.WithContext(apikey.WithPrincipal(req.Context(), p))
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	require.Equal(t, len(bigBody), receivedLen,
		"handler debe recibir el body completo intacto")
}

// Sanity: CleanupExpired existe y retorna tipos correctos.
func TestCleanupExpired_Signature(t *testing.T) {
	i := &Idempotency{Pool: nil}
	// No llamamos (panic con nil pool); validamos la firma.
	require.NotNil(t, i)
	_ = i.CleanupExpired
	_ = context.Background
}
