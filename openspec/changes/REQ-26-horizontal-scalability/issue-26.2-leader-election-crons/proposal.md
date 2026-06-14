# Proposal: issue-26.2-leader-election-crons

## Intención

Leader election via Postgres advisory locks para que crons singleton ejecuten 1x aún con N pods.

## Scope

- Helper `leader.Acquire(ctx, name)` / `Release(name)`
- Cron worker chequea leadership antes de ejecutar
- Heartbeat updates `system_crons.last_heartbeat_at`
- Métricas + alertas
- Forced takeover si stale

## Enfoque

1. `pg_try_advisory_lock(hash(cron_name))` non-blocking
2. Si lock obtenido + execute + release al final
3. Heartbeat cada 10s mientras corre
4. Connection death → auto-release del lock por Postgres

## Riesgos

- Conn ephemeral en pgbouncer transaction-pool: usar conn dedicado para lock (session-pool)
- Split-brain: aceptable porque Postgres es coord central

## Testing

- 5 goroutines simultáneas → 1 obtiene lock
- Cierre conn → otro adquiere
- Heartbeat updates visibles
- Métrica leader gauge correcta
