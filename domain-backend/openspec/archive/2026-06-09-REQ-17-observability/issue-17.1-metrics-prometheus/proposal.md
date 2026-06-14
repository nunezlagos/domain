# Proposal: issue-17.1-metrics-prometheus

## Intención

Instrumentar Domain con métricas Prometheus expuestas en `/metrics` (puerto separado por defecto), cubriendo runtime Go, HTTP, DB pool y métricas de dominio (runs, tokens, costo, skills). Habilitar SLOs y alertas operacionales.

## Scope

**Incluye:**
- Dependencia `github.com/prometheus/client_golang`
- Middleware HTTP que registra `http_requests_total` y `http_request_duration_seconds`
- Instrumentación pgx v5 para pool stats (`domain_db_pool_in_use`, `domain_db_pool_acquired_total`)
- Métricas de dominio: runs, tokens, costo, skills
- Endpoint `/metrics` en puerto configurable (`DOMAIN_METRICS_PORT`, default 9090)
- Basic auth opcional en /metrics
- Documentación `docs/observability/metrics.md`

**No incluye:**
- Dashboards Grafana (otra HU futura)
- Alerting rules (configuración de Prometheus, no del binario)
- Tracing ni logs (issue-17.2, issue-17.3)

## Enfoque técnico

1. Registry global con `prometheus.NewRegistry()` para evitar globals problemáticos en tests
2. Middleware HTTP wrapping mux/router con `promhttp.InstrumentHandlerCounter` y `Duration`
3. Hook pgx con `pgxpool.Config.AfterConnect` o stats periódicos vía goroutine
4. Helper `metrics.RecordRun(type, status, durationS)` invocado desde services
5. Cardinalidad acotada por convención + tests que fallan si se agrega un label con `_id` regex

## Riesgos

- Cardinalidad: mitigada por revisión de labels en code review + test automático
- Overhead histogram: usar buckets razonables (50ms..30s default)
- Puerto separado expone superficie adicional: bind a interfaz interna y/o basic auth

## Testing

- Test HTTP middleware: hacer request → métrica counter incrementa
- Test cardinalidad: scrapear /metrics, regex que falla si encuentra `_id="<uuid>"`
- Test pool: stats reflejan inserts (in_use sube y baja)
