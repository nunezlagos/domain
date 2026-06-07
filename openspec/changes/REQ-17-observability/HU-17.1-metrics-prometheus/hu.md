# HU-17.1-metrics-prometheus

**Origen:** `REQ-17-observability`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** operador de la plataforma Domain
**Quiero** un endpoint `/metrics` en formato Prometheus exponiendo métricas de runtime, HTTP, DB y dominio
**Para** poder monitorear salud, latencia, error rate y consumo en producción con alertas y SLOs

## Criterios de aceptación

### Escenario 1: Endpoint /metrics expone métricas estándar

```gherkin
Dado que el servidor Domain está corriendo
Cuando hago GET /metrics
Entonces la respuesta es Content-Type "text/plain; version=0.0.4"
Y contiene las métricas runtime Go: `go_goroutines`, `go_memstats_alloc_bytes`, `process_cpu_seconds_total`
Y contiene `http_requests_total{method,path,status}` con counter
Y contiene `http_request_duration_seconds{method,path}` con histogram
```

### Escenario 2: Métricas de dominio expuestas

```gherkin
Dado que el sistema procesa runs de agentes y flows
Cuando consulto /metrics
Entonces existen métricas custom:
  | métrica                                 | tipo      | labels                       |
  | domain_runs_total                       | counter   | type, status, org_id         |
  | domain_run_duration_seconds             | histogram | type, status                 |
  | domain_tokens_total                     | counter   | provider, model, direction   |
  | domain_cost_usd_total                   | counter   | provider, model, org_id      |
  | domain_db_pool_in_use                   | gauge     | -                            |
  | domain_skill_executions_total           | counter   | skill_slug, status           |
```

### Escenario 3: Métricas con baja cardinalidad

```gherkin
Dado que /metrics se scrapea cada 15s
Cuando inspecciono labels
Entonces ningún label contiene IDs únicos como user_id, request_id, run_id (alta cardinalidad)
Y los labels permitidos están documentados en `docs/observability/metrics.md`
```

### Escenario 4: Configuración del endpoint

```gherkin
Dado que existe `DOMAIN_METRICS_ENABLED=true` y `DOMAIN_METRICS_PORT=9090`
Cuando arranca el servidor
Entonces /metrics se sirve en puerto separado 9090 (no en el API principal)
Y opcionalmente puede protegerse con basic auth vía `DOMAIN_METRICS_AUTH_USER/PASSWORD`
```

## Análisis breve

- **Qué pide realmente:** Cliente Prometheus oficial Go (`prometheus/client_golang`) instrumentando handlers HTTP, pool pgx, runs/cost. Endpoint separado para evitar exposición pública del API y permitir scraping interno.
- **Módulos sospechados:** `internal/observability/metrics/`, instrumentación en `internal/http/middleware/`, `internal/store/postgres/`, `internal/service/agent.go`, `internal/service/flow.go`.
- **Riesgos:** Cardinalidad explosiva si se incluyen IDs únicos. Overhead de instrumentación.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Revisar codebase (no existe instrumentación previa)
- [ ] Validar lista de métricas con producto/oncall

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
