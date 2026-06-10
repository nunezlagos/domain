// Package leader — issue-26.2 leader election via Postgres advisory locks.
//
// Cada pod intenta tomar un advisory lock global (pg_try_advisory_lock).
// El primero gana y es leader. Si la connection DB muere (graceful shutdown
// o crash), Postgres libera el lock automáticamente y otro pod lo toma en
// su próximo poll.
//
// Uso: solo el leader corre tasks que NO deben duplicarse (cron scheduler,
// outbound webhook delivery, idempotency cleanup).
package leader

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Election struct {
	Pool       *pgxpool.Pool
	LockKey    int64         // identificador único del lock
	PollPeriod time.Duration // default 10s
	Logger     *slog.Logger

	mu   sync.Mutex
	conn *pgxpool.Conn
}

// TryAcquire intenta tomar el lock. Idempotente: si ya lo teníamos y la
// connection sigue viva, devuelve true sin re-adquirir.
func (e *Election) TryAcquire(ctx context.Context) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.conn != nil {
		if err := e.conn.Ping(ctx); err == nil {
			return true, nil
		}
		e.conn.Release()
		e.conn = nil
	}

	conn, err := e.Pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	var got bool
	err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", e.LockKey).Scan(&got)
	if err != nil {
		conn.Release()
		return false, err
	}
	if !got {
		conn.Release()
		return false, nil
	}
	e.conn = conn
	return true, nil
}

// Release libera el lock + connection.
func (e *Election) Release(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.conn == nil {
		return
	}
	_, _ = e.conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", e.LockKey)
	e.conn.Release()
	e.conn = nil
}

// RunAsLeader bloquea polling hasta ganar lock, después ejecuta fn(ctx).
// fn debe respetar ctx para shutdown. Cuando fn retorna, libera lock.
func (e *Election) RunAsLeader(ctx context.Context, fn func(ctx context.Context)) {
	period := e.PollPeriod
	if period == 0 {
		period = 10 * time.Second
	}
	logger := e.Logger
	if logger == nil {
		logger = slog.Default()
	}

	for {
		select {
		case <-ctx.Done():
			e.Release(context.Background())
			return
		default:
		}

		got, err := e.TryAcquire(ctx)
		if err != nil {
			logger.Warn("leader election error", slog.Any("err", err))
			time.Sleep(period)
			continue
		}
		if !got {
			logger.Debug("not leader; waiting")
			time.Sleep(period)
			continue
		}

		logger.Info("acquired leader lock", slog.Int64("lock_key", e.LockKey))
		fn(ctx)
		e.Release(context.Background())
		logger.Info("released leader lock")
		return
	}
}

// Lock keys registry (evita colisiones de uso entre features).
const (
	LockKeyCronScheduler      int64 = 1001
	LockKeyOutboundWebhooks   int64 = 1002
	LockKeyIdempotencyCleanup int64 = 1003
	LockKeyInvitationsExpire  int64 = 1004
	LockKeyDBStatsSnapshot    int64 = 1005
)
