// HU-17.1 metrics-prometheus unit tests.

package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_RegistersStandardCollectors(t *testing.T) {
	r := New()
	// Counters Prom solo aparecen tras Inc/Observe; gauges siempre.
	// Forzamos increment para confirmar registration.
	r.HTTPRequestsTotal.WithLabelValues("GET", "/x", "200").Inc()
	r.AgentRunsTotal.WithLabelValues("chat", "ok").Inc()
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	// Go runtime collectors siempre presentes
	require.Contains(t, body, "go_goroutines")
	require.Contains(t, body, "go_memstats_alloc_bytes")
	// Custom registered
	require.Contains(t, body, "domain_http_requests_total")
	require.Contains(t, body, "domain_db_pool_in_use") // gauge always
	require.Contains(t, body, "domain_agent_runs_total")
}

func TestHTTPMiddleware_RecordsRequest(t *testing.T) {
	r := New()
	handler := r.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/users", nil))

	metrics := scrape(t, r)
	require.Contains(t, metrics, `domain_http_requests_total{method="POST",path="/api/v1/users",status="201"} 1`)
}

func TestHTTPMiddleware_NormalizesUUIDPath(t *testing.T) {
	r := New()
	handler := r.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/users/01234567-89ab-cdef-0123-456789abcdef", nil))

	metrics := scrape(t, r)
	require.Contains(t, metrics, `path="/api/v1/users/:id"`)
	require.NotContains(t, metrics, "01234567-89ab")
}

func TestHTTPMiddleware_NormalizesNumericPath(t *testing.T) {
	r := New()
	handler := r.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/projects/42/items", nil))

	metrics := scrape(t, r)
	require.Contains(t, metrics, `path="/api/v1/projects/:n/items"`)
	require.NotContains(t, metrics, "/projects/42/")
}

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"/health":                        "/health",
		"/api/v1/users/abc-def":          "/api/v1/users/abc-def", // no UUID format
		"/api/v1/users/01234567-89ab-cdef-0123-456789abcdef": "/api/v1/users/:id",
		"/api/v1/projects/42":            "/api/v1/projects/:n",
		"/":                              "/",
	}
	for in, want := range cases {
		got := normalizePath(in)
		require.Equalf(t, want, got, "normalizePath(%q)", in)
	}
}

func TestDomainCounters_Increment(t *testing.T) {
	r := New()
	r.AgentRunsTotal.WithLabelValues("chat", "completed").Inc()
	r.LLMTokensTotal.WithLabelValues("openai", "gpt-4", "input").Add(100)
	r.CostUSDTotal.WithLabelValues("openai", "gpt-4").Add(0.05)
	r.SkillExecsTotal.WithLabelValues("summarize", "success").Inc()

	metrics := scrape(t, r)
	require.Contains(t, metrics, `domain_agent_runs_total{status="completed",type="chat"} 1`)
	require.Contains(t, metrics, `domain_llm_tokens_total{direction="input",model="gpt-4",provider="openai"} 100`)
	require.Contains(t, metrics, `domain_cost_usd_total{model="gpt-4",provider="openai"} 0.05`)
	require.Contains(t, metrics, `domain_skill_executions_total{skill_slug="summarize",status="success"} 1`)
}

func TestBasicAuth_BlocksMissingAuth(t *testing.T) {
	r := New()
	r.AgentRunsTotal.WithLabelValues("t", "ok").Inc()

	handler := basicAuth(r.Handler(), "user", "pass")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, `Basic realm="metrics"`, rec.Header().Get("WWW-Authenticate"))
}

func TestBasicAuth_AcceptsValidCreds(t *testing.T) {
	r := New()
	handler := basicAuth(r.Handler(), "user", "pass")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.SetBasicAuth("user", "pass")
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestBasicAuth_RejectsBadCreds(t *testing.T) {
	r := New()
	handler := basicAuth(r.Handler(), "user", "pass")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.SetBasicAuth("user", "wrong")
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Sabotaje: HU-17.1 cardinality linter — el body /metrics NO debe tener `_id="<uuid>"`.
func TestSabotage_NoUUIDLabelsInMetrics(t *testing.T) {
	r := New()
	// Simular tráfico
	handler := r.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET",
		"/api/v1/users/01234567-89ab-cdef-0123-456789abcdef", nil))

	metrics := scrape(t, r)
	// regex que detecta UUIDs en labels
	uuidLabel := regexp.MustCompile(`_id="[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"`)
	require.False(t, uuidLabel.MatchString(metrics),
		"cardinality violation: UUID en label encontrado en /metrics body")
}

// Sabotaje: helper para verificar histogram buckets razonables.
func TestSabotage_HistogramBucketsAreReasonable(t *testing.T) {
	r := New()
	// Trigger observe para que buckets aparezcan
	handler := r.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))

	rec2 := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec2, httptest.NewRequest("GET", "/metrics", nil))
	body := rec2.Body.String()

	require.Contains(t, body, `domain_http_request_duration_seconds_bucket{`)
	require.Contains(t, body, `le="30"`)
	_ = strings.Repeat // keep import even si no se usa
}

// Helper: scrape metrics y retorna body como string.
func scrape(t *testing.T, r *Registry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return string(body)
}
