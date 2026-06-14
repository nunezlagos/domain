# Proposal: issue-26.7-cache-invalidation-patterns

## Intención

Patrón uniforme de cache invalidation cross-pod usando Postgres LISTEN/NOTIFY. Helper Go + SQL trigger + convención naming.

## Scope

- `internal/cache/distributed/` helper
- SQL function `create_cache_invalidation_trigger(table)`
- Reconnect logic + flush cache on reconnect
- Dedupe window 100ms
- Métricas

## Riesgos

- NOTIFY 8KB limit: payload mínimo (id + op + org_id)
- Pgbouncer txn-pool: usar conn dedicada session
- Out-of-order: invalidation es idempotente (delete cache entry)

## Testing

- Update tabla → todos los pods reciben NOTIFY
- Cache invalidado en todos
- Reconnect → flush all
- Dedupe 100ms window
- Métricas observables
