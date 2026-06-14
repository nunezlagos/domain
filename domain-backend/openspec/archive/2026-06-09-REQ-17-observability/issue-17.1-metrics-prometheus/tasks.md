# Tasks: issue-17.1-metrics-prometheus

## Backend

- [ ] **metrics-001**: Agregar dependencia `github.com/prometheus/client_golang`
- [ ] **metrics-002**: Crear `internal/observability/metrics/registry.go` con `New()` y registro Go default collectors
- [ ] **metrics-003**: Crear `metrics/http.go` con `Middleware(next)` que registra requests + duration
- [ ] **metrics-004**: Crear `metrics/pgx.go` con `InstrumentPool(pool, ctx)` que publica gauges del pool cada 5s
- [ ] **metrics-005**: Crear `metrics/domain.go` con helpers `RecordRun`, `RecordTokens`, `RecordCost`, `RecordSkill`
- [ ] **metrics-006**: Crear `metrics/server.go` con `Serve(ctx, addr, authUser, authPass)` levantando HTTP separado
- [ ] **metrics-007**: Wire en `cmd/domain-mcp/main.go`: arrancar metrics server detrás de feature flag
- [ ] **metrics-008**: Wire middleware en `internal/http/router.go`
- [ ] **metrics-009**: Llamar `RecordRun/RecordTokens/RecordCost` desde `service/agent.go`, `service/flow.go`

## Config

- [ ] **cfg-001**: Agregar `DOMAIN_METRICS_*` al sistema de config (issue-01.2)
- [ ] **cfg-002**: Defaults seguros: enabled=true, bind=127.0.0.1, port=9090

## Tests

- [ ] **test-001**: Unit middleware HTTP (counter + histogram)
- [ ] **test-002**: Unit pool gauges
- [ ] **test-003**: Unit cardinalidad: regex `(_id|request_id)="[^"]+"` debe encontrar 0 matches
- [ ] **test-004**: Integration server: levantar, hacer requests, scrape, parsear con `expfmt`
- [ ] **sabotaje-001**: Agregar label `user_id=` → test-003 falla → revertir

## Docs

- [ ] **docs-001**: Crear `docs/observability/metrics.md` con lista de métricas, labels permitidos, ejemplos de queries PromQL
- [ ] **docs-002**: Ejemplos de alerting rules en `docs/observability/alerts.example.yml`

## Cierre

- [ ] Smoke manual: `curl http://127.0.0.1:9090/metrics` en dev
