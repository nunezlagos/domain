# Design: issue-26.2-leader-election-crons

## Helper

```go
// internal/leader/leader.go
type Leader struct {
  pool *pgxpool.Pool
  name string
  conn *pgxpool.Conn  // dedicated session-level conn (NOT pgbouncer txn-pool)
}

func Acquire(ctx context.Context, pool *pgxpool.Pool, name string) (*Leader, bool, error) {
  conn, _ := pool.Acquire(ctx)  // acquire dedicated
  var got bool
  conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", hash(name)).Scan(&got)
  if !got {
    conn.Release()
    return nil, false, nil
  }
  return &Leader{pool, name, conn}, true, nil
}

func (l *Leader) Release() {
  l.conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", hash(l.name))
  l.conn.Release()
}
```

## Cron worker pattern

```go
func RunCron(ctx context.Context, name string, fn func(ctx) error) {
  ticker := time.NewTicker(interval)
  for {
    select {
    case <-ctx.Done(): return
    case <-ticker.C:
      ld, got, _ := leader.Acquire(ctx, pool, name)
      if !got { slog.Debug("not leader, skip"); continue }
      hbCtx, hbCancel := context.WithCancel(ctx)
      go heartbeat(hbCtx, pool, name, 10*time.Second)
      err := fn(ctx)
      hbCancel()
      ld.Release()
      if err != nil { slog.ErrorContext(ctx, "cron failed", "err", err) }
    }
  }
}
```

## Important: pgbouncer

Advisory locks tied to **session**, no transaction. PgBouncer transaction-pool ROMPE esto.

Solución: leader uses **separate connection** que bypasea pgbouncer (directo a Postgres) o `session pool_mode` para específico user.

```
[databases]
domain_leader = host=postgres port=5432 dbname=domain pool_mode=session
```

App config dual: `DOMAIN_DATABASE_URL` (txn pool) + `DOMAIN_DATABASE_URL_LEADER` (session pool).

## TDD plan

1. 5 goroutines acquire mismo name → 1 succeeds
2. Leader conn close → another acquires
3. Heartbeat updates DB
4. Métrica gauge per cron
5. Forced takeover si stale heartbeat
