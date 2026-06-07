# RFC 0004: Cost Observability vs Metrics Prometheus Boundary

**Status:** accepted
**Date:** 2026-06-07
**Related:** REQ-15 cost-observability, REQ-17 observability (HU-17.1 metrics-prometheus)

## Contexto

Dos sistemas exponen información relacionada a costos/tokens:

- **REQ-15 Cost Observability**: tracking de tokens/costo por run/agent/flow/org, analytics, alertas
- **REQ-17 HU-17.1 Metrics Prometheus**: métricas `domain_tokens_total`, `domain_cost_usd_total` con labels

Riesgo: dos tablas + dos sistemas de alertas + confusión sobre source of truth.

## Decisión

Separación por **propósito y consumer**:

### REQ-15 Cost Observability — **producto / facturación**

- **Audience**: usuarios finales, admins de org, billing
- **Granularidad**: por run, por user, por entity_id
- **Retention**: indefinida (parte del producto)
- **Storage**: tabla `cost_logs` particionada por mes
- **Access**: API + UI con RBAC scoping (user ve sólo su org)
- **Queries típicas**:
  - "¿cuánto gasté este mes?"
  - "¿qué agent es más caro?"
  - "Top 10 users por consumo"
  - "Trend de gasto últimos 90 días"
- **Alertas**: thresholds configurables por org (`80% del plan` → notif), atadas a billing/plans
- **Source of truth para facturación**

### REQ-17 HU-17.1 Metrics Prometheus — **operacional / SRE**

- **Audience**: SRE, on-call, dashboards Grafana
- **Granularidad**: aggregated por labels (provider, model, org_id), **sin IDs únicos** (cardinality)
- **Retention**: típicamente 15-90d (config Prometheus)
- **Storage**: time-series Prometheus
- **Access**: `/metrics` scrape (puerto separado, auth opcional)
- **Queries típicas** (PromQL):
  - `rate(domain_tokens_total[5m])` por provider
  - `sum by (model) (rate(domain_cost_usd_total[1h]))`
  - SLOs de costo (cost/req)
- **Alertas**: PromQL-driven en Alertmanager, foco en anomalías operacionales (spike repentino, provider degraded)
- **Source of truth para SRE**

## Tabla comparativa

| dimensión | cost_logs (REQ-15) | metrics (REQ-17.1) |
|-----------|--------------------|--------------------|
| Audience | usuarios, billing | SRE, oncall |
| Granularidad | por run_id, user_id | aggregated, low-cardinality |
| Storage | Postgres particionado | Prometheus TSDB |
| Retention | indefinida | 15-90d |
| Access | API/UI con RBAC | /metrics scrape |
| Alertas | thresholds producto | PromQL anomaly |
| Auth | API key + RBAC | basic auth opcional |
| Source of truth | facturación | SRE |

## Source of truth: ¿quién manda?

**Conflicto potencial:** cost_logs dice "$120" y Prometheus suma "$118.7"

**Regla:** **cost_logs es source of truth** porque:
1. Es persisted con per-row detail
2. Se usa para facturar
3. Prometheus puede tener gaps por scrape misses

Si discrepan: investigar Prometheus (gaps, drops). cost_logs gana.

## Cómo se relacionan

```
event "agent_run.completed"
  ├─ INSERT cost_logs (run_id, user_id, tokens_in, tokens_out, cost_usd, ...)
  └─ metrics.RecordCost(provider, model, org_id, cost_usd)  // counter increment
```

**Un solo evento, dos sinks distintos**, ambos populados desde el mismo service layer.

## Implementación

```go
// internal/service/cost.go
func RecordCost(ctx context.Context, ev CostEvent) error {
  // 1. Persist cost_logs (source of truth)
  if err := store.InsertCostLog(ctx, ev); err != nil {
    return err  // facturación crítica, falla loud
  }
  // 2. Update Prometheus counters (best-effort)
  metrics.RecordCost(ev.Provider, ev.Model, ev.OrgID, ev.CostUSD)
  // 3. Check thresholds (REQ-15.3 alertas)
  go checkUsageAlerts(ctx, ev)
  return nil
}
```

## Cuándo NO duplicar

| concepto | cost_logs | metrics |
|----------|-----------|---------|
| Tokens por modelo agregado | rollup table o vista | ✓ time series |
| Costo de un run específico | ✓ | ✗ (cardinalidad) |
| Costo por hora del día | aggregation query | ✓ time series |
| Alerta "user X excedió plan" | ✓ (RBAC, plan-aware) | ✗ |
| Alerta "provider OpenAI lento" | ✗ | ✓ (latency metrics) |

## Anti-patrones

- ❌ Exponer cost_logs como /metrics endpoint (cardinalidad explota)
- ❌ Usar Prometheus para facturar (gaps, retention)
- ❌ Duplicar lógica de threshold alerting en ambos
- ❌ Mostrar Prometheus en UI usuario (audience equivocada)

## Consecuencias

**Positivas:**
- Cada audience tiene la herramienta correcta
- Cardinality acotada en Prometheus
- Source of truth claro para disputes
- Facturación basada en datos persisted

**Negativas:**
- Doble write per evento (mitigado: metrics es in-memory atomic, sin I/O)
- Drift posible (mitigado: cron diario reconcile detecta y alerta)

## Open questions

- ¿Cron de reconciliation diario entre cost_logs y métricas Prometheus? Yes, recomendado HU separada en REQ-15.
- ¿Cost forecasting (predicción) basado en histórico? Yes, futuro HU.
