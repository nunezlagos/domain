# Design: HU-17.1-metrics-prometheus

## Decisión arquitectónica

**Cliente:** `github.com/prometheus/client_golang` (oficial).
**Registry:** custom registry (`prometheus.NewRegistry()`) — no `DefaultRegisterer`, para evitar contaminación entre tests y procesos.
**Endpoint:** puerto separado configurable (default 9090), no en el API principal (3000/8080).
**Buckets de histograma HTTP:** `[0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30]` segundos.

## Alternativas descartadas

- **OpenMetrics nativo sin client_golang:** mucho boilerplate, perdemos `promhttp` helpers
- **Métricas en /admin/metrics del API principal:** mezcla auth/concerns, dificulta scrape interno
- **Push gateway:** no aplica, Domain es servicio long-running pull-based

## Componentes

```
internal/observability/metrics/
  registry.go     → New() *prometheus.Registry, MustRegister(...)
  http.go         → Middleware(handler) http.Handler
  pgx.go          → InstrumentPool(pool *pgxpool.Pool, ctx)
  domain.go       → RecordRun, RecordTokens, RecordCost, RecordSkill
  server.go       → Serve(ctx, addr, auth) error
```

## Variables de entorno

| var | default | descripción |
|-----|---------|-------------|
| DOMAIN_METRICS_ENABLED | true | Habilita endpoint |
| DOMAIN_METRICS_BIND | 127.0.0.1 | Interfaz de bind |
| DOMAIN_METRICS_PORT | 9090 | Puerto |
| DOMAIN_METRICS_AUTH_USER | "" | Basic auth user (vacío = sin auth) |
| DOMAIN_METRICS_AUTH_PASSWORD | "" | Basic auth password |

## TDD plan

1. Test unit: middleware aumenta counter en cada request
2. Test unit: histograma registra duración
3. Test unit: regex de cardinalidad sobre `/metrics` body
4. Test integration: levantar server + pool, hacer queries, verificar gauges
5. Test sabotaje: agregar label `user_id="..."` → test cardinalidad falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Cardinalidad explosiva | Media | Alto | Lint test sobre /metrics |
| Overhead instrumentación | Baja | Bajo | Buckets razonables, registry lazy |
| Exposición pública sin auth | Media | Medio | Bind 127.0.0.1 + basic auth opcional |
