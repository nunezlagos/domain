// Package metrics — HU-17.1 metrics-prometheus.
//
// Custom registry (no DefaultRegisterer) para isolation entre tests y procesos.
// Registry expone /metrics en puerto separado configurable (DOMAIN_METRICS_PORT).
// Cardinalidad acotada: labels enum + linter test detecta `_id="<uuid>"`.
package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry encapsula prom.Registerer + collectors custom de Domain.
type Registry struct {
	reg *prometheus.Registry

	// HTTP
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// DB pool
	DBPoolInUse      prometheus.Gauge
	DBPoolAcquired   prometheus.Counter
	DBQueryDuration  *prometheus.HistogramVec

	// Dominio
	AgentRunsTotal    *prometheus.CounterVec
	AgentRunDuration  *prometheus.HistogramVec
	LLMTokensTotal    *prometheus.CounterVec
	CostUSDTotal      *prometheus.CounterVec
	SkillExecsTotal   *prometheus.CounterVec
}

// New crea Registry con todas las métricas registradas.
func New() *Registry {
	reg := prometheus.NewRegistry()
	// Runtime Go + process collectors (estándar Prometheus)
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	r := &Registry{reg: reg}

	r.HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_http_requests_total",
			Help: "Total HTTP requests por method/path/status",
		},
		[]string{"method", "path", "status"},
	)
	r.HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "domain_http_request_duration_seconds",
			Help:    "Duración HTTP requests",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"method", "path"},
	)

	r.DBPoolInUse = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_db_pool_in_use",
		Help: "Conexiones pgx en uso",
	})
	r.DBPoolAcquired = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "domain_db_pool_acquired_total",
		Help: "Total conn acquires",
	})
	r.DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "domain_db_query_duration_seconds",
			Help:    "Duración queries pgx",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		},
		[]string{"operation"},
	)

	r.AgentRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_agent_runs_total",
			Help: "Total agent runs por tipo/status",
		},
		[]string{"type", "status"},
	)
	r.AgentRunDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "domain_agent_run_duration_seconds",
			Help:    "Duración agent runs",
			Buckets: []float64{0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		},
		[]string{"type", "status"},
	)
	r.LLMTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_llm_tokens_total",
			Help: "Tokens LLM por provider/model/direction",
		},
		[]string{"provider", "model", "direction"},
	)
	r.CostUSDTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_cost_usd_total",
			Help: "Costo USD por provider/model",
		},
		[]string{"provider", "model"},
	)
	r.SkillExecsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_skill_executions_total",
			Help: "Skill executions por slug/status",
		},
		[]string{"skill_slug", "status"},
	)

	reg.MustRegister(
		r.HTTPRequestsTotal,
		r.HTTPRequestDuration,
		r.DBPoolInUse,
		r.DBPoolAcquired,
		r.DBQueryDuration,
		r.AgentRunsTotal,
		r.AgentRunDuration,
		r.LLMTokensTotal,
		r.CostUSDTotal,
		r.SkillExecsTotal,
	)
	return r
}

// Handler retorna http.Handler para /metrics con auth opcional.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          r.reg,
	})
}

// Prometheus retorna el Registry crudo (para tests + advanced wiring).
func (r *Registry) Prometheus() *prometheus.Registry { return r.reg }

// Middleware HTTP que registra requests + duration.
// Path normalization: usa template path (ej `/api/v1/users/:id`) NO el actual con UUID.
func (r *Registry) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		// Wrap response writer para capturar status
		ww := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, req)
		path := normalizePath(req.URL.Path)
		r.HTTPRequestsTotal.WithLabelValues(req.Method, path, strconv.Itoa(ww.status)).Inc()
		r.HTTPRequestDuration.WithLabelValues(req.Method, path).Observe(time.Since(start).Seconds())
	})
}

// normalizePath bucketiza paths con IDs para evitar cardinality explosion.
// Reemplaza UUIDs y números por placeholders.
func normalizePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		// UUID-ish: 36 chars with dashes
		if len(part) == 36 && strings.Count(part, "-") == 4 {
			parts[i] = ":id"
			continue
		}
		// Pure number
		if _, err := strconv.Atoi(part); err == nil {
			parts[i] = ":n"
		}
	}
	return strings.Join(parts, "/")
}

// statusRecorder captura status code escrito por handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Serve arranca un HTTP server separado para /metrics.
// addr: "127.0.0.1:9090". user/pass opcional (basic auth si user != "").
func (r *Registry) Serve(addr, user, pass string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", basicAuth(r.Handler(), user, pass))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

func basicAuth(h http.Handler, user, pass string) http.Handler {
	if user == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}
