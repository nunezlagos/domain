# REQ-28-arquitectura-interna: Refactor estructural de capa interna

**Estado:** activo
**Creado:** 2026-06-11
**Fase:** F1 (transversal — corre paralelo a otras HUs)

## Descripción

Deuda técnica estructural identificada en el codebase analysis. Aborda 8 áreas que hoy impactan productividad al cambiar features o agregar nuevas: ausencia de interfaces de repositorio, construcción sin validación, cross-org guard manual repetido, error matching frágil contra strings de Postgres, errores silenciados, circuit breaker con dead code, goroutines sin lifecycle, y timer leaks en retry loops.

El enfoque es **Strangler Fig**: cada HU agrega la abstracción al lado del código existente, migra callers de a uno, y solo elimina el camino viejo cuando no queda ningún consumidor. Behavior-preserving excepto donde se corrige comportamiento incorrecto (HU-28.5, HU-28.6, HU-28.7) — esos casos tienen sabotaje explícito.

## Criterios de éxito

- Repository interfaces definidas para los 5 services más acoplados (flow, agent, observation, session, project)
- Constructores con validación reemplazan struct literals en wiring de `cmd/domain/main.go`
- Cross-org guard extraído a helper + middleware de principal elimina ~30 repeticiones en handlers
- `pgerrcode.UniqueViolation` reemplaza `strings.Contains(err.Error(), "duplicate key")` en 30+ lugares
- `_ = json.NewEncoder(w).Encode(body)` ya no ignora errores en toda la capa HTTP
- `_ = s.Audit.Record(...)` ya no ignora errores en servicios
- Circuit breaker `CompleteStream` registra fallos mid-stream
- Webhooks tienen context con timeout + `sync.WaitGroup` para lifecycle controlado
- `time.After` reemplazado por `time.NewTimer` con `defer timer.Stop()` en retry loops
- Cero regresiones en suite de tests existente

## HUs hijas

| HU | Prioridad | Tipo | Descripción |
|----|-----------|------|-------------|
| HU-28.1-repository-interfaces | alta | refactor | Repository interfaces para flow, agent, observation, session, project |
| HU-28.2-constructors-validation | alta | refactor | Constructores públicos con validación, fields privados |
| HU-28.3-middleware-principal-crossorg | alta | refactor | Middleware de principal + helper authorizeOrg |
| HU-28.4-pgerrcode-error-codes | media | refactor | pgerrcode en vez de string matching |
| HU-28.5-fix-ignored-errors | alta | fix | JSON encode + audit + write errors ya no se silencian |
| HU-28.6-fix-circuit-breaker-stream | media | fix | CompleteStream registra fallos mid-stream |
| HU-28.7-webhook-goroutine-lifecycle | media | fix | Context con timeout + WaitGroup en dispatch |
| HU-28.8-timeafter-timertimer | baja | refactor | time.NewTimer con stop diferido |
