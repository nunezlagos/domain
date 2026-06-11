# Distributed Locks — Operación

> issue-26.3 — `internal/dlock/`

Locks distribuidos sobre **advisory locks de Postgres** para coordinar
workers entre pods (crons leader-only, jobs exclusivos, migraciones de datos).

## API

```go
m := &dlock.Manager{Pool: pools.App, Metrics: metricsReg}

// No-bloqueante
lk, ok, err := m.TryAcquire(ctx, "cron_scheduler")
if ok { defer lk.Release(ctx) }

// Con espera (polling 200ms) hasta maxWait → dlock.ErrTimeout
lk, err := m.Acquire(ctx, "backfill-2026-06", 30*time.Second)

// Conveniencia: ejecuta fn solo si obtuvo el lock
executed, err := m.WithLock(ctx, "daily-report", func(ctx context.Context) error { ... })
```

- El key string se hashea estable a int64 (SHA-256 primeros 8 bytes).
- El lock vive en una **conexión dedicada** del pool retenida hasta `Release`.
- Si el proceso muere, **Postgres libera el lock automáticamente** al cerrar
  la session (verificado por `TestConnDie_AutoReleases`).

## Métricas

- `domain_dlock_acquire_total{key, result}` — result: `acquired|busy|error`
- `domain_dlock_held_duration_seconds{key}` — histograma de tenencia

Alertar si `held_duration` p99 crece sostenido (lock leak / job colgado) o si
`busy` domina (contención).

## Requisitos de infraestructura

- El pool debe ser **session-mode**. Con PgBouncer en transaction-pooling los
  advisory locks NO funcionan (la conn se comparte entre transacciones).
  Conectar dlock directo a Postgres o a un pool session-mode dedicado
  (ver `deploy/pgbouncer/README.md`, issue-25.1).

## Anti-patterns

- ❌ **Lock dentro de transacción** (`pg_advisory_xact_lock` semantics):
  esta API usa session locks; no mezclar con locks transaccionales del mismo key.
- ❌ **Olvidar Release** — la conn queda retenida del pool hasta que el
  proceso muera. Siempre `defer lk.Release(ctx)` o usar `WithLock`.
- ❌ **Keys de alta cardinalidad** (`run-<uuid>`): rompe métricas y no
  aporta — los advisory locks son para recursos lógicos acotados.
- ❌ **Usar dlock como mutex de datos** — para exclusión de filas usar
  `SELECT ... FOR UPDATE`; dlock es para coordinación de procesos.
- ❌ **Esperas largas con Acquire** en handlers HTTP — bloquea la request;
  usar TryAcquire + 409/retry del lado del cliente.
- ❌ **Asumir fairness** — el polling no garantiza orden FIFO entre waiters.
