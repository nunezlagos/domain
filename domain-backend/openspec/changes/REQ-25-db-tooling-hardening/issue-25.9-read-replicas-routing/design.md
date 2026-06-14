# Design: issue-25.9-read-replicas-routing

## Replication setup

- Streaming async replication (no synchronous; latency budget aceptable)
- Primary: `wal_level=replica`, `max_wal_senders=10`
- Replica: `hot_standby=on`, `primary_conninfo=...`

## Helm chart support

```yaml
postgresql:
  primary:
    persistence: { size: 100Gi }
  replica:
    enabled: true
    replicaCount: 1
    persistence: { size: 100Gi }
    targetSessionAttrs: read-only
```

## App pgx config

```go
type DB struct {
  Primary  *pgxpool.Pool
  Replica  *pgxpool.Pool
  lagInfo  *atomic.Pointer[LagInfo]
}

func (d *DB) Read(ctx context.Context, fn func(pgx.Querier) error) error {
  if d.lagInfo.Load().IsHealthy() {
    return fn(d.Replica)
  }
  return fn(d.Primary)  // fallback
}

func (d *DB) Write(ctx context.Context, fn func(pgx.Tx) error) error {
  return d.Primary.BeginTxFunc(ctx, pgx.TxOptions{}, fn)
}

func (d *DB) ReadFresh(ctx context.Context, fn func(pgx.Querier) error) error {
  return fn(d.Primary)
}
```

## Lag monitor

```go
// goroutine cada 30s
for { 
  var lag float64
  d.Replica.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp())`).Scan(&lag)
  metrics.SetReplicationLag(lag)
  d.lagInfo.Store(&LagInfo{Seconds: lag, HealthyThreshold: 10.0})
  time.Sleep(30 * time.Second)
}
```

## TDD plan

1. Replica recibe streaming (pg_is_in_recovery=true)
2. db.Read targetea replica
3. db.Write targetea primary
4. lag metric publicada
5. lag fake >10s → fallback primary
6. db.ReadFresh siempre primary
7. Replica caída → fallback transparent
