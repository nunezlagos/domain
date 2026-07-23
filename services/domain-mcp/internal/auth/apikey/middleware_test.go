package apikey

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// Fake resolver para tests.
type fakeResolver struct {
	expected  string
	principal *Principal
	err       error
}

func (f *fakeResolver) Resolve(_ context.Context, plaintext string) (*Principal, error) {
	if f.err != nil {
		return nil, f.err
	}
	if plaintext != f.expected {
		return nil, ErrUnauthorized
	}
	return f.principal, nil
}

func nextEchoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := FromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-User-Id", p.UserID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

// fakeFailureLogger captura las llamadas al sink de auth_events (DOMAINSERV-82 H1).
type fakeFailureLogger struct {
	calls []failCall
}
type failCall struct{ reason, ip, ua string }

func (f *fakeFailureLogger) LogAPIKeyAuthFailure(_ context.Context, reason, ip, ua string) {
	f.calls = append(f.calls, failCall{reason, ip, ua})
}

func TestMiddleware_APIKeyInvalidCredentials_LogsAuthFailure(t *testing.T) {
	fl := &fakeFailureLogger{}
	pt, _, _, _ := Generate("live")
	m := &Middleware{Resolver: &fakeResolver{err: errors.New("DB down")}, FailureLogger: fl}
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("Authorization", "Bearer "+pt)
	req.Header.Set("User-Agent", "curl/8.0")
	m.Wrap(nextEchoHandler()).ServeHTTP(httptest.NewRecorder(), req)

	require.Len(t, fl.calls, 1)
	require.Contains(t, fl.calls[0].reason, "invalid_credentials")
	require.Contains(t, fl.calls[0].reason, "/api/v1/foo")
	require.Equal(t, "curl/8.0", fl.calls[0].ua)
	require.NotEmpty(t, fl.calls[0].ip)
	// el token NUNCA debe aparecer en los campos logueados (policy secrets-redaction)
	require.NotContains(t, fl.calls[0].reason, pt)
	require.NotContains(t, fl.calls[0].ua, pt)
	require.NotContains(t, fl.calls[0].ip, pt)
}

func TestMiddleware_APIKeyInvalidFormat_LogsAuthFailure(t *testing.T) {
	fl := &fakeFailureLogger{}
	m := &Middleware{Resolver: &fakeResolver{}, FailureLogger: fl}
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("Authorization", "Bearer not_an_api_key")
	m.Wrap(nextEchoHandler()).ServeHTTP(httptest.NewRecorder(), req)

	require.Len(t, fl.calls, 1)
	require.Contains(t, fl.calls[0].reason, "invalid_format")
}

func TestMiddleware_APIKeySuccess_NoAuthFailure(t *testing.T) {
	fl := &fakeFailureLogger{}
	pt, _, _, _ := Generate("live")
	m := &Middleware{
		Resolver:      &fakeResolver{expected: pt, principal: &Principal{UserID: "u", OrganizationID: "o"}},
		FailureLogger: fl,
	}
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("Authorization", "Bearer "+pt)
	m.Wrap(nextEchoHandler()).ServeHTTP(httptest.NewRecorder(), req)

	require.Empty(t, fl.calls)
}

func TestMiddleware_NoBearer_401(t *testing.T) {
	r := &fakeResolver{}
	m := &Middleware{Resolver: r}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_InvalidFormat_401(t *testing.T) {
	m := &Middleware{Resolver: &fakeResolver{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("Authorization", "Bearer not_an_api_key")
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_ValidKey_PropagatesPrincipal(t *testing.T) {
	pt, _, _, _ := Generate("live")
	r := &fakeResolver{
		expected:  pt,
		principal: &Principal{UserID: "user-1", OrganizationID: "org-1", APIKeyID: "k-1", Role: "member"},
	}
	m := &Middleware{Resolver: r}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("Authorization", "Bearer "+pt)
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "user-1", rec.Header().Get("X-User-Id"))
}

func TestMiddleware_ResolverError_401(t *testing.T) {
	pt, _, _, _ := Generate("live")
	r := &fakeResolver{err: errors.New("DB down")}
	m := &Middleware{Resolver: r}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	req.Header.Set("Authorization", "Bearer "+pt)
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_Allowlist_NoAuthRequired(t *testing.T) {
	m := &Middleware{Resolver: &fakeResolver{}, Allowlist: []string{"/health"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	called := false
	m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

// Sabotaje: 401 body es uniforme (anti-enumeration issue-02.7).
func TestSabotage_401Body_Uniform(t *testing.T) {
	m := &Middleware{Resolver: &fakeResolver{err: ErrUnauthorized}}
	for _, h := range []string{"", "Bearer foo", "Bearer domk_live_INVALIDxxxxxxxxxxxxxxxxx"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/x", nil)
		if h != "" {
			req.Header.Set("Authorization", h)
		}
		m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)

		body := rec.Body.String()
		require.Contains(t, body, "unauthorized")
		require.NotContains(t, body, "key")    // no leak detalle
		require.NotContains(t, body, "bearer") // no leak qué falló
	}
}
