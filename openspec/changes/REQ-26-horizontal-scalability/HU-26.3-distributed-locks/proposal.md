# Proposal: HU-26.3-distributed-locks

## Intención

Helper genérico `dlock` reutilizable basado en Postgres advisory locks para coordinar operaciones cross-pod.

## Scope

- `internal/dlock/` con API TryAcquire/Acquire/Release
- Session-pool dedicada para locks (no pgbouncer txn)
- Métricas Prometheus
- Documentación con anti-patterns

## Enfoque

Postgres advisory locks. Key = sha256(name) truncated to int64.

## Riesgos

- PgBouncer txn-pool: usar conn dedicada session-pool
- Deadlock entre múltiples locks: documentar lock ordering convention

## Testing

- TryAcquire concurrent
- Conn death auto-release
- Acquire con wait timeout
- Métricas observables
