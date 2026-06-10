# Observability Conventions — Domain

3 pilares: **logs** (slog), **metrics** (Prometheus), **traces** (OpenTelemetry). Correlacionados por `trace_id` + `request_id`.

Implementado por REQ-17 (issue-17.1 metrics, issue-17.2 traces, issue-17.3 logs).

## Logs

### Formato
- JSON en prod, text legible en dev
- Campos estándar SIEMPRE presentes:
  - `time` (ISO 8601)
  - `level` (debug/info/warn/error)
  - `msg`
  - `request_id` (si HTTP request)
  - `trace_id`, `span_id` (si OTel span activo)
  - `user_id`, `org_id`, `project_id` (si contexto auth)

### Cuándo logear

| nivel | qué |
|-------|-----|
| Debug | parameter values, query results count, cache hit/miss (dev only) |
| Info | request received, run started/completed, login, important state transitions |
| Warn | retry, degraded mode (replica down → fallback primary), rate limit hit, deprecated API used |
| Error | non-recoverable error, panic, dependency caída unrecoverable |

### Cuándo NO logear

- Full request bodies (puede tener secrets/PII)
- API keys, passwords, OTP codes (ver security.md keys prohibidas)
- Polling/heartbeat noise repetitivo
- Validation errors triviales (`required field missing`) — solo si frecuente y operacional

### Helpers

```go
logger := logging.FromContext(ctx)  // tiene request_id, trace_id, user_id, org_id automatic
logger.Info("user created",
  slog.String("user_id", u.ID.String()),    // OK
  slog.String("email", u.Email),             // PROHIBIDO — linter falla
  slog.Int("api_keys_count", len(u.APIKeys)),
)
```

## Métricas

### Naming

```
domain_<subsystem>_<metric>_<unit>
```

Ejemplos:
- `domain_http_requests_total{method,path,status}` (counter)
- `domain_http_request_duration_seconds{method,path}` (histogram)
- `domain_agent_runs_total{type,status,org_id}` (counter)
- `domain_db_pool_in_use` (gauge)
- `domain_llm_tokens_total{provider,model,direction}` (counter)
- `domain_cost_usd_total{provider,model,org_id}` (counter)

### Cardinalidad

- Labels acotados: status (5-10 values), method (5), path (~30 routes), provider (4), model (~20)
- NUNCA labels con: user_id, request_id, run_id, observation_id, project_id de prod (millones)
- `org_id` aceptable solo si <10000 orgs esperados
- issue-17.1 lint test `_id="<uuid>"` regex en `/metrics` body

### Histograms buckets

HTTP duration: `[0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30]` (seconds)
LLM call duration: `[0.5, 1, 2.5, 5, 10, 30, 60, 120]` (seconds)
DB query duration: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5]` (seconds)

### Counters

- Sufijo `_total` siempre (`requests_total`, no `requests`)
- Solo monotónicamente crecientes; usar gauge para valores que pueden bajar

### Gauges

- Estado actual: `pool_in_use`, `goroutines`, `queue_depth`
- Pueden subir y bajar
- Sin sufijo `_total`

## Traces

### Setup

- W3C TraceContext propagation
- Sampler: `ParentBased(TraceIDRatioBased(0.1))` default; always-on para errores
- Resource: `service.name=domain-mcp`, `service.version`, `deployment.environment`

### Span naming

- HTTP: `HTTP <METHOD> <route_pattern>` (`HTTP GET /api/v1/observations/:id`)
- DB: `pg.query <table>` o `pg.<operation>`
- LLM: `llm.<provider>.<model>`
- Skill: `skill.<slug>`
- Agent: `agent.<slug>`

### Attributes

Whitelist (issue-17.2 `SafeAttrs()`):
- `http.method`, `http.status_code`, `http.route`
- `db.statement` redacted (no values)
- `db.rows_affected`
- `llm.model`, `llm.input_tokens`, `llm.output_tokens`, `llm.cost_usd`
- `agent.slug`, `skill.slug`
- `org.id`, `user.id` (UUIDs OK)

NUNCA en attributes:
- `user.email`, `user.rut`
- `observation.content`, `prompt.body`
- API keys, secrets

## Correlación

Cada request HTTP propaga:
- `request_id` UUID v4 generado al entry, header `X-Request-Id`
- `trace_id` si tracing activo (continúa de upstream o genera)

Logs incluyen ambos; traces tienen trace_id; metrics no (cardinality).

Para debug: `request_id` o `trace_id` → Loki query → ver flow → Tempo/Jaeger query con trace_id → ver spans.

## Health

- `/health` liveness — 200 si proceso vivo
- `/health/ready` readiness — 200 si DB + deps healthy
- `/health/startup` startup probe — 200 cuando boot done

## Dashboards y alerts

- Dashboards Grafana versionados en `deploy/grafana/dashboards/`
- AlertManager rules en `deploy/prometheus/alerts/`
- Alertas core:
  - Error rate > 1% por 5min
  - p99 latency > 2s por 10min
  - DB pool > 80% sostenido
  - DB replication lag > 10s
  - Backup failed
  - Heartbeat absent > 60s
  - Cost spend > 80% plan in 10min (anomaly)

## Anti-patterns prohibidos

- ❌ `log.Printf` o `fmt.Println` (usar `slog`)
- ❌ Logging dentro de hot loops sin throttling
- ❌ Métricas con label de alta cardinalidad
- ❌ Traces con attributes sensibles
- ❌ Health endpoint que pega a DB en cada call (cache 5s)
- ❌ Sin readiness probe → ELB ruta tráfico a pod no-listo
