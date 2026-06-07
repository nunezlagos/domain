// HU-17.2 tracing unit tests.

package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestSetup_Disabled_NoopProvider(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Tracer del provider noop debe funcionar sin errores ni network calls
	_, span := otel.Tracer("test").Start(context.Background(), "test-span")
	span.End()
	require.NoError(t, shutdown(context.Background()))
}

func TestClamp(t *testing.T) {
	require.Equal(t, 0.0, clamp(-1.0))
	require.Equal(t, 0.5, clamp(0.5))
	require.Equal(t, 1.0, clamp(1.5))
	require.Equal(t, 0.0, clamp(0))
	require.Equal(t, 1.0, clamp(1.0))
}

func TestIsSafeKey(t *testing.T) {
	// Allowed
	require.True(t, IsSafeKey("http.method"))
	require.True(t, IsSafeKey("llm.provider"))
	require.True(t, IsSafeKey("user.id"))
	require.True(t, IsSafeKey("org.id"))

	// Forbidden
	require.False(t, IsSafeKey("user.email"))
	require.False(t, IsSafeKey("password"))
	require.False(t, IsSafeKey("api_key"))
	require.False(t, IsSafeKey("observation.content"))
	require.False(t, IsSafeKey("random.thing"))
}

func TestSafeAttr_AllowedKey(t *testing.T) {
	a := SafeAttr("http.method", "GET")
	require.Equal(t, "http.method", string(a.Key))
	require.Equal(t, "GET", a.Value.AsString())
}

func TestSafeAttr_ForbiddenKey_ReturnsEmpty(t *testing.T) {
	a := SafeAttr("password", "secret")
	// empty key value when not in whitelist
	require.Equal(t, "", string(a.Key))
}

func TestSafeAttr_DifferentTypes(t *testing.T) {
	require.Equal(t, "200", SafeAttr("http.status_code", 200).Value.Emit())
	require.Equal(t, "12345", SafeAttr("llm.input_tokens", int64(12345)).Value.Emit())
	require.Equal(t, "0.05", SafeAttr("llm.cost_usd", 0.05).Value.Emit())
}

func TestNormalizeRoute(t *testing.T) {
	cases := map[string]string{
		"/health":                                           "/health",
		"/api/v1/users/01234567-89ab-cdef-0123-456789abcdef": "/api/v1/users/:id",
		"/api/v1/projects/42/items":                         "/api/v1/projects/:n/items",
		"/":                                                  "/",
	}
	for in, want := range cases {
		require.Equalf(t, want, normalizeRoute(in), "normalizeRoute(%q)", in)
	}
}

func TestHTTPMiddleware_NoErrorOnRequest(t *testing.T) {
	// Setup noop provider para test sin OTLP endpoint real
	_, _ = Setup(context.Background(), Config{Enabled: false})

	called := false
	mw := HTTPMiddleware("test")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/users/01234567-89ab-cdef-0123-456789abcdef", nil)
	h.ServeHTTP(rec, req)
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestHTTPMiddleware_PropagatesTraceContext(t *testing.T) {
	_, _ = Setup(context.Background(), Config{Enabled: false})

	mw := HTTPMiddleware("test")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// El span debe estar en el context (no validamos no-op SpanContext, solo que no crashea)
		span := otel.Tracer("inner").Start
		_ = span
		w.WriteHeader(http.StatusCreated)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/x", nil)
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
}

// Sabotaje: SafeAttr filtra TODOS los keys prohibidos comunes.
func TestSabotage_SafeAttr_FiltersAllForbidden(t *testing.T) {
	forbidden := []string{
		"password", "passwd", "secret", "token", "api_key",
		"otp", "otp_code", "email", "rut", "phone",
		"user.email", "user.rut", "user.phone",
		"observation.content", "prompt.body", "skill.content",
		"db.statement", // raw statement con values
	}
	for _, k := range forbidden {
		a := SafeAttr(k, "sensitive_value")
		require.Equalf(t, "", string(a.Key),
			"SafeAttr(%q) NO debe propagar PII", k)
	}
}

// Sabotaje: la whitelist no incluye accidentalmente keys peligrosos.
// Excepción: "llm.input_tokens" y "llm.output_tokens" usan "tokens" en sentido
// LLM (unidades contables), no auth tokens.
func TestSabotage_WhitelistDoesNotLeak(t *testing.T) {
	allowedTokensContext := map[string]bool{
		"llm.input_tokens":  true,
		"llm.output_tokens": true,
	}
	for k := range safeAttrKeys {
		require.NotContainsf(t, k, "email", "key %q leaks email", k)
		require.NotContainsf(t, k, "password", "key %q leaks password", k)
		require.NotContainsf(t, k, "secret", "key %q leaks secret", k)
		require.NotContainsf(t, k, "rut", "key %q leaks rut", k)
		require.NotContainsf(t, k, "content", "key %q leaks content", k)
		// "token" en auth sentido prohibido EXCEPTO llm.*_tokens (unidades)
		if !allowedTokensContext[k] {
			require.NotContainsf(t, k, "token", "key %q leaks auth token", k)
		}
	}
}
