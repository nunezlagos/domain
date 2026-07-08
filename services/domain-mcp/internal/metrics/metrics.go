// Package metrics — issue-17.1 metrics-prometheus.
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


	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec


	DBPoolInUse      *prometheus.GaugeVec
	DBPoolIdle       *prometheus.GaugeVec
	DBPoolTotal      *prometheus.GaugeVec
	DBPoolAcquired   prometheus.Counter
	DBQueryDuration  *prometheus.HistogramVec


	ReplicationLagSeconds prometheus.Gauge
	ReplicaQueriesTotal   prometheus.Counter
	ReplicaFallbackTotal  prometheus.Counter


	DBConnectionsActive            prometheus.Gauge
	DBConnectionsIdle              prometheus.Gauge
	DBConnectionsIdleInTransaction prometheus.Gauge
	DBLongestQuerySeconds          prometheus.Gauge
	DBLockWaitsTotal               *prometheus.CounterVec
	DBTableDeadTuples              *prometheus.GaugeVec


	AgentRunsTotal    *prometheus.CounterVec
	AgentRunDuration  *prometheus.HistogramVec
	LLMTokensTotal    *prometheus.CounterVec
	CostUSDTotal      *prometheus.CounterVec
	SkillExecsTotal    *prometheus.CounterVec
	FlowStepRetriesTotal *prometheus.CounterVec
	PprofAccessTotal   prometheus.Counter
	SlowQueriesTotal   *prometheus.CounterVec


	HeartbeatWatcherStuckTotal *prometheus.CounterVec // labels: org_id, phase, reason
	HeartbeatWatcherTicksTotal *prometheus.CounterVec // labels: result (ok|leader_skip|error)

	AgentRunsOrphanTotal *prometheus.CounterVec // labels: org_id, reason
	OrphanAuditTicksTotal *prometheus.CounterVec // labels: result

	OrchestratorRunsTotal       *prometheus.CounterVec   // labels: mode, status
	OrchestratorPhaseDuration   *prometheus.HistogramVec // labels: phase, mode
	OrchestratorPhaseResultsTotal *prometheus.CounterVec // labels: phase, mode, result (completed|failed|shape_contract_unmet|tool_contract_unmet|required_save_unmet)
	OrchestratorConfirmsTotal   *prometheus.CounterVec   // labels: confirmed (true|false)
	OrchestratorRequiredSaveMissingTotal *prometheus.CounterVec // labels: phase, save_type


	FlowHeartbeatAgeSeconds prometheus.Gauge // age del heartbeat más reciente en flow_runs


	FlowRunCancelledByMaxDuration *prometheus.CounterVec // labels: org_id


	DlockAcquireTotal *prometheus.CounterVec   // labels: key, result (acquired|busy|error)
	DlockHeldSeconds  *prometheus.HistogramVec // labels: key


	DispatchTotal    *prometheus.CounterVec   // labels: source, target_type, result
	DispatchDuration *prometheus.HistogramVec // labels: source, target_type


	MCPToolCallsTotal  *prometheus.CounterVec   // labels: tool, status
	MCPToolDuration    *prometheus.HistogramVec // labels: tool
	MCPCacheHitsTotal  prometheus.Counter
	MCPCacheMissesTotal prometheus.Counter
	MCPCacheSize       prometheus.Gauge
	TicketsLockedActive prometheus.Gauge
	SSESubscribers     prometheus.Gauge
}

// New crea Registry con todas las métricas registradas.
func New() *Registry {
	reg := prometheus.NewRegistry()

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
	r.FlowStepRetriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_flow_step_retries_total",
			Help: "Flow step retries por flow_slug/step_id",
		},
		[]string{"flow_slug", "step_id"},
	)
	r.PprofAccessTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "domain_debug_pprof_accessed_total",
		Help: "Total accesos a /debug/pprof/*",
	})
	r.HeartbeatWatcherStuckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_heartbeat_watcher_stuck_total",
			Help: "Flow run steps detectados stuck por heartbeat timeout (issue-08.11)",
		},
		[]string{"org_id", "phase", "reason"},
	)
	r.HeartbeatWatcherTicksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_heartbeat_watcher_ticks_total",
			Help: "Ticks del heartbeat-watcher cron (issue-08.11)",
		},
		[]string{"result"},
	)
	r.AgentRunsOrphanTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_agent_runs_orphan_total",
			Help: "Agent runs orphan detectados (sin flow_run_id ni standalone flag) — issue-08.12",
		},
		[]string{"org_id", "reason"},
	)
	r.OrphanAuditTicksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_orphan_audit_ticks_total",
			Help: "Ticks del orphan-runs-audit cron (issue-08.12)",
		},
		[]string{"result"},
	)
	r.SlowQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_db_slow_queries_total",
			Help: "Slow queries detectadas por threshold",
		},
		[]string{"threshold_ms"},
	)


	r.OrchestratorRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_orchestrator_runs_total",
			Help: "Runs del orquestador SDD iniciados, por mode y status terminal",
		},
		[]string{"mode", "status"},
	)
	r.OrchestratorPhaseDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "domain_orchestrator_phase_duration_seconds",
			Help:    "Duración (s) reportada por el cliente para cada fase SDD",
			Buckets: []float64{0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"phase", "mode"},
	)
	r.OrchestratorPhaseResultsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_orchestrator_phase_results_total",
			Help: "Resultados de fase reportados (completed|failed|shape_contract_unmet|tool_contract_unmet|required_save_unmet)",
		},
		[]string{"phase", "mode", "result"},
	)
	r.OrchestratorConfirmsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_orchestrator_confirms_total",
			Help: "Eventos de domain_orchestrate_confirm (D1) — accepted o rejected",
		},
		[]string{"confirmed"},
	)
	r.OrchestratorRequiredSaveMissingTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_orchestrator_required_save_missing_total",
			Help: "Veces que un cliente reportó phase_result sin un suggested_save Required (D5)",
		},
		[]string{"phase", "save_type"},
	)

	r.FlowHeartbeatAgeSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_flow_heartbeat_age_seconds",
		Help: "Edad del heartbeat más reciente entre flow_runs running (issue-09.6)",
	})

	r.FlowRunCancelledByMaxDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_flow_run_cancelled_by_max_duration_total",
			Help: "Flow runs cancelados por exceder max_flow_duration_seconds per-org (issue-33.3)",
		},
		[]string{"org_id"},
	)


	r.DlockAcquireTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_dlock_acquire_total",
			Help: "Intentos de acquire de distributed locks por resultado",
		},
		[]string{"key", "result"},
	)
	r.DlockHeldSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "domain_dlock_held_duration_seconds",
			Help:    "Duración que un distributed lock estuvo tomado",
			Buckets: []float64{0.01, 0.1, 0.5, 1, 5, 30, 60, 300, 1800},
		},
		[]string{"key"},
	)


	r.DispatchTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_dispatch_total",
			Help: "Total dispatches por source, target_type y result (success|failed)",
		},
		[]string{"source", "target_type", "result"},
	)
	r.DispatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "domain_dispatch_duration_seconds",
			Help:    "Duración de dispatch en segundos",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 30, 60, 300},
		},
		[]string{"source", "target_type"},
	)


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
		r.FlowStepRetriesTotal,
		r.PprofAccessTotal,
		r.SlowQueriesTotal,
		r.HeartbeatWatcherStuckTotal,
		r.HeartbeatWatcherTicksTotal,
		r.AgentRunsOrphanTotal,
		r.OrphanAuditTicksTotal,
		r.OrchestratorRunsTotal,
		r.OrchestratorPhaseDuration,
		r.OrchestratorPhaseResultsTotal,
		r.OrchestratorConfirmsTotal,
		r.OrchestratorRequiredSaveMissingTotal,
		r.FlowHeartbeatAgeSeconds,
		r.FlowRunCancelledByMaxDuration,
		r.DlockAcquireTotal,
		r.DlockHeldSeconds,
		r.DispatchTotal,
		r.DispatchDuration,
	)
	r.initREQ70()
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

// RegisterDispatchTotal crea y registra DispatchTotal lazy.
// Útil cuando un Registry se construyó sin esa métrica (e.g., un
// Registry de test mínimo) y se necesita observarla después.
func (r *Registry) RegisterDispatchTotal() *prometheus.CounterVec {
	if r.DispatchTotal != nil {
		return r.DispatchTotal
	}
	r.DispatchTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_dispatch_total",
			Help: "Total dispatches (lazy-registered)",
		},
		[]string{"source", "target_type", "result"},
	)
	r.reg.MustRegister(r.DispatchTotal)
	return r.DispatchTotal
}

// RegisterDispatchDuration crea y registra DispatchDuration lazy.
func (r *Registry) RegisterDispatchDuration() *prometheus.HistogramVec {
	if r.DispatchDuration != nil {
		return r.DispatchDuration
	}
	r.DispatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "domain_dispatch_duration_seconds",
			Help:    "Duración de dispatch (lazy-registered)",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 30, 60, 300},
		},
		[]string{"source", "target_type"},
	)
	r.reg.MustRegister(r.DispatchDuration)
	return r.DispatchDuration
}

// initREQ70 registra métricas custom para MCP tools + cache + SSE.
// Se llama desde New() después del bloque principal.
func (r *Registry) initREQ70() {
	r.MCPToolCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "domain_mcp_tool_calls_total",
			Help: "Total MCP tool calls por tool y resultado (ok|error|cache_hit).",
		},
		[]string{"tool", "status"},
	)
	r.MCPToolDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "domain_mcp_tool_duration_seconds",
			Help:    "Latencia de MCP tool handlers (sin contar cache hits).",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5, 30},
		},
		[]string{"tool"},
	)
	r.MCPCacheHitsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "domain_mcp_cache_hits_total",
		Help: "Total hits del query cache MCP (REQ-67).",
	})
	r.MCPCacheMissesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "domain_mcp_cache_misses_total",
		Help: "Total misses del query cache MCP (REQ-67).",
	})
	r.MCPCacheSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_mcp_cache_size",
		Help: "Tamaño actual del query cache MCP (entries).",
	})
	r.TicketsLockedActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_tickets_locked_active",
		Help: "Cantidad de tickets con lock vigente (no expirado).",
	})
	r.SSESubscribers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "domain_sse_subscribers",
		Help: "Cantidad actual de subscribers SSE conectados (REQ-69).",
	})
	r.reg.MustRegister(
		r.MCPToolCallsTotal, r.MCPToolDuration,
		r.MCPCacheHitsTotal, r.MCPCacheMissesTotal, r.MCPCacheSize,
		r.TicketsLockedActive, r.SSESubscribers,
	)
}

// Middleware HTTP que registra requests + duration.
// Path normalization: usa template path (ej `/api/v1/users/:id`) NO el actual con UUID.
func (r *Registry) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()

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

		if len(part) == 36 && strings.Count(part, "-") == 4 {
			parts[i] = ":id"
			continue
		}

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
