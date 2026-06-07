# HU-26.6-backpressure-queue

**Origen:** `REQ-26-horizontal-scalability`
**Prioridad tentativa:** media
**Tipo:** hardening

## Historia de usuario

**Como** operador
**Quiero** que las colas internas tengan cap + shed-load + 429 con Retry-After cuando saturadas
**Para** no llenar memoria/DB cuando demand spike

## Aplicado a

- `notification_deliveries` queue
- `flow_runs` pending pool
- `agent_runs` pending pool
- `outbound_webhook_deliveries` queue
- `embedding_queue`
- `import_jobs` queue
- `export_jobs` queue

## Criterios de aceptación

### Escenario 1: Queue depth métrica

```gherkin
Dado que cada queue tabla tiene gauge `domain_queue_depth{queue}` con count pending
Cuando se publica
Entonces alerta si >5000 sostenido 5min
Y dashboard Grafana muestra trend
```

### Escenario 2: Shed-load en API

```gherkin
Dado que `flow_runs` queue tiene >5000 pending
Cuando llega POST /flows/:id/run
Entonces 429 `{"error":"queue_full","retry_after_seconds":60}`
Y header `Retry-After: 60`
Y métrica `domain_queue_shed_total{queue}` incrementa
```

### Escenario 3: Per-org quota

```gherkin
Dado que org tiene plan Free con max 100 pending runs
Cuando intenta encolar el 101
Entonces 429 "org_queue_limit_exceeded"
```

### Escenario 4: Worker pool size adapter

```gherkin
Dado que queue depth > threshold
Cuando worker pool ya en máximo
Entonces métrica alerta para scale-up manual
Y no se overfetch (workers no toman más de lo procesable)
```

### Escenario 5: Drop oldest opcional

```gherkin
Dado que queue tipo "non-critical" (telemetry) está full
Cuando llega nuevo
Entonces se descarta más viejo (FIFO con drop)
Y métrica `domain_queue_dropped_total{queue,reason}`
```

## Análisis breve

- **Qué pide:** caps + métricas + 429 + per-org quota + drop policy declarada
- **Esfuerzo:** S
- **Riesgos:** false positives si threshold mal calibrado
