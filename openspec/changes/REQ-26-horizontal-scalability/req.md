# REQ-26-horizontal-scalability: Escalabilidad horizontal: stateless invariant, leader election, distributed locks, graceful shutdown, circuit breaker LLM, backpressure, cache invalidation patterns.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F5

## Descripción

Patrones y mecanismos que permiten a Domain escalar horizontalmente con N pods sin race conditions, sin duplicar work, sin perder requests en SIGTERM, sin tumbar la plataforma si LLM provider hipo.

## Criterios de éxito

- Stateless invariant enforced por linter (no state in-memory crítico)
- Leader election Postgres-backed garantiza que crons corren 1x aún con N pods
- Distributed locks reutilizables (advisory locks Postgres) para coordinación cross-pod
- Graceful shutdown drena HTTP + workers + pool en SIGTERM con timeout
- Circuit breaker LLM provider: tripped tras N errores, fallback documentado, half-open recovery
- Backpressure en colas con cap configurable + shed-load
- Cache invalidation patterns formalizados (LISTEN/NOTIFY estándar para todas las invalidaciones cross-pod)

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-26.1-stateless-invariant | proposed | Linter Go que detecta state crítico in-memory; whitelist explícita |
| issue-26.2-leader-election-crons | proposed | Leader election Postgres advisory lock para crons singleton |
| issue-26.3-distributed-locks | proposed | Helper distributed-locks Postgres + pattern docs |
| issue-26.4-graceful-shutdown | proposed | SIGTERM handler drain HTTP + workers + pool con timeout |
| issue-26.5-circuit-breaker-llm | proposed | CB per provider/model con fallback, half-open, métricas |
| issue-26.6-backpressure-queue | proposed | Caps en colas + shed-load + 429 con Retry-After |
| issue-26.7-cache-invalidation-patterns | proposed | LISTEN/NOTIFY estándar cross-pod + helper + tests |
