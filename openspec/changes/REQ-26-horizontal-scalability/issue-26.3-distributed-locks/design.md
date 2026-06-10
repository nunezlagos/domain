# Design: issue-26.3-distributed-locks

## API

```go
package dlock

type Lock struct { /* unexported */ }

// Non-blocking try
func TryAcquire(ctx context.Context, pool *pgxpool.Pool, key string) (*Lock, bool, error)

// Blocking with timeout
func Acquire(ctx context.Context, pool *pgxpool.Pool, key string, maxWait time.Duration) (*Lock, error)

func (l *Lock) Release() error
```

## Implementation

```go
func TryAcquire(ctx, pool, key) (*Lock, bool, error) {
  // pool MUST be session-pool aware
  conn, _ := pool.Acquire(ctx)
  k := hashKey(key)
  var got bool
  err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", k).Scan(&got)
  if !got { conn.Release(); return nil, false, err }
  metrics.DLockAcquired(key)
  return &Lock{conn, k, time.Now()}, true, nil
}
```

## Key hashing

```go
// stable BIGINT from string
func hashKey(s string) int64 {
  h := sha256.Sum256([]byte("domain.dlock:" + s))
  return int64(binary.BigEndian.Uint64(h[:8]))
}
```

## TDD plan

1. 2 goroutines TryAcquire same key → 1 wins
2. Release → another can acquire
3. Conn die → auto release verifiable
4. Acquire con wait + timeout
5. Different keys NO collision
6. Métricas observables
