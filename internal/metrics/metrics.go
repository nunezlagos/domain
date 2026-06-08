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
	DBPoolInUse      *prometheus.GaugeVec
	DBPoolIdle       *prometheus.GaugeVec
	DBPoolTotal      *prometheus.GaugeVec
	DBPoolAcquired   prometheus.Counter
	DBQueryDuration  *prometheus.HistogramVec

	// Replicación (HU-25.9)
	ReplicationLagSeconds prometheus.Gauge
	ReplicaQueriesTotal   prometheus.Counter
	ReplicaFallbackTotal  prometheus.Counter

	// DB monitoring (HU-25.12)
	DBConnectionsActive            prometheus.Gauge
	DBConnectionsIdle              prometheus.Gauge
	DBConnectionsIdleInTransaction prometheus.Gauge
	DBLongestQuerySeconds          prometheus.Gauge
	DBLockWaitsTotal               *prometheus.CounterVec
	DBTableDeadTuples              *prometheus.GaugeVec

	// Dominio
	AgentRunsTotal    *prometheus.CounterVec
	AgentRunDuration  *prometheus.HistogramVec
	LLMTokensTotal    *prometheus.CounterVec
	CostUSDTotal      *prometheus.CounterVec
	SkillExecsTotal   *prometheus.CounterVec
	PprofAccessTotal  prometheus.Counter
	SlowQueriesTotal  *prometheus.CounterVec
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

	r.DBPoolInUse = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "domain_db_pool_in_use",
		Help: "Conexiones pgx en uso",
	}, []string{"pool"})
	r.DBPoolIdle = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "domain_db_pool_idle",
		Help: "Conexiones pgx idle",
	}, []string{"pool"})
	r.DBPoolTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "domain_db_pool_total",
		Help: "Conexiones pgx totales (max)",
	}, []string{"pool"})
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
	r.PprofAccessTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "domain_debug_pprof_accessed_total",
		Help: "Total accesos a /debug/pprof/*",
	})
	r.SlowQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_db_slow_queries_total",
			Help: "Slow queries detectadas por threshold",
		},
		[]string{"threshold_ms"},
	)

	// HU-25.9 read-replicas
	r.ReplicationLagSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_db_replication_lag_seconds",
		Help: "Replication lag en segundos (0 si no hay replica)",
	})
	r.ReplicaQueriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "domain_db_replica_queries_total",
		Help: "Queries ruteadas a replica",
	})
	r.ReplicaFallbackTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "domain_db_replica_fallback_total",
		Help: "Fallbacks a primary por replica degradada",
	})

	// HU-25.12 locks-vacuum
	r.DBConnectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_db_connections_active",
		Help: "Conexiones activas en pg_stat_activity",
	})
	r.DBConnectionsIdle = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_db_connections_idle",
		Help: "Conexiones idle en pg_stat_activity",
	})
	r.DBConnectionsIdleInTransaction = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_db_connections_idle_in_transaction",
		Help: "Conexiones idle in transaction",
	})
	r.DBLongestQuerySeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_db_longest_query_seconds",
		Help: "Duracion de la query activa mas larga",
	})
	r.DBLockWaitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_db_lock_waits_total",
			Help: "Lock waits detectados por wait type y tabla",
		},
		[]string{"wait_type", "table"},
	)
	r.DBTableDeadTuples = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "domain_db_table_dead_tuples",
			Help: "Dead tuples por tabla",
		},
		[]string{"table"},
	)

	reg.MustRegister(
		r.HTTPRequestsTotal,
		r.HTTPRequestDuration,
		r.DBPoolInUse,
		r.DBPoolIdle,
		r.DBPoolTotal,
		r.DBPoolAcquired,
		r.DBQueryDuration,
		r.ReplicationLagSeconds,
		r.ReplicaQueriesTotal,
		r.ReplicaFallbackTotal,
		r.DBConnectionsActive,
		r.DBConnectionsIdle,
		r.DBConnectionsIdleInTransaction,
		r.DBLongestQuerySeconds,
		r.DBLockWaitsTotal,
		r.DBTableDeadTuples,
		r.AgentRunsTotal,
		r.AgentRunDuration,
		r.LLMTokensTotal,
		r.CostUSDTotal,
		r.SkillExecsTotal,
		r.PprofAccessTotal,
		r.SlowQueriesTotal,
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
