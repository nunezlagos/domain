// Package dlock — HU-26.3 distributed locks reutilizables vía advisory locks Postgres.
//
// API:
//   - TryAcquire: no-bloqueante, retorna inmediatamente (acquired bool).
//   - Acquire: polling hasta acquire o ctx/timeout (maxWait).
//   - Lock.Release(): libera + cierra conn.
//
// Implementación: pg_try_advisory_lock(key int8) tomado en una conn dedicada
// (no en transacción) para que el lock persista mientras el caller lo mantenga.
// Postgres libera automáticamente el lock al cerrar la session (escenario 2).
//
// El "key" string se hashea a int64 con SHA-256 mod 2^63 — colisión astronómicamente
// improbable, suficiente para nombres de feature locks.
package dlock

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrTimeout retornado por Acquire si maxWait vence antes de obtener el lock.
	ErrTimeout = errors.New("dlock_acquire_timeout")
)

// PollInterval default del Acquire con espera.
const PollInterval = 200 * time.Millisecond

// HashKey convierte un nombre lógico a int64 estable para pg_try_advisory_lock.
// SHA-256 primeros 8 bytes interpretados como int64.
func HashKey(name string) int64 {
	h := sha256.Sum256([]byte(name))
	u := binary.BigEndian.Uint64(h[:8])
	// Mask al rango int64 positivo no es necesario; pg acepta int8 signed.
	return int64(u)
}

// Manager crea locks sobre un pool. El pool debe ser session-mode capable
// (NO PgBouncer transaction-pool) — Acquire saca una conn dedicada que retiene.
type Manager struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

// Lock representa un advisory lock mantenido por una conn dedicada.
// Llamar Release() libera el lock y devuelve la conn al pool.
type Lock struct {
	conn      *pgxpool.Conn
	key       int64
	keyName   string
	acquired  time.Time
	logger    *slog.Logger
	released  bool
}

// TryAcquire intenta tomar el lock sin esperar. Retorna (nil, false, nil) si está ocupado.
func (m *Manager) TryAcquire(ctx context.Context, keyName string) (*Lock, bool, error) {
	key := HashKey(keyName)
	conn, err := m.Pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire conn: %w", err)
	}
	var got bool
	err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&got)
	if err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("try_advisory_lock: %w", err)
	}
	if !got {
		conn.Release()
		return nil, false, nil
	}
	if m.Logger != nil {
		m.Logger.DebugContext(ctx, "dlock acquired",
			slog.String("key", keyName))
	}
	return &Lock{
		conn: conn, key: key, keyName: keyName,
		acquired: time.Now(), logger: m.Logger,
	}, true, nil
}

// Acquire espera hasta obtener el lock o que venza maxWait.
// Polling cada PollInterval (200ms default). Retorna ErrTimeout si excede.
func (m *Manager) Acquire(ctx context.Context, keyName string, maxWait time.Duration) (*Lock, error) {
	deadline := time.Now().Add(maxWait)
	for {
		lk, ok, err := m.TryAcquire(ctx, keyName)
		if err != nil {
			return nil, err
		}
		if ok {
			return lk, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%w: key=%s after %s", ErrTimeout, keyName, maxWait)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(PollInterval):
		}
	}
}

// Release libera el lock + devuelve la conn al pool.
// Es idempotente: llamar 2x no panickea.
func (l *Lock) Release(ctx context.Context) error {
	if l == nil || l.released {
		return nil
	}
	l.released = true
	defer l.conn.Release()
	_, err := l.conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", l.key)
	if l.logger != nil {
		l.logger.DebugContext(ctx, "dlock released",
			slog.String("key", l.keyName),
			slog.Duration("held", time.Since(l.acquired)))
	}
	if err != nil {
		return fmt.Errorf("advisory_unlock: %w", err)
	}
	return nil
}

// HeldFor retorna cuánto tiempo lleva tomado el lock (para métricas).
func (l *Lock) HeldFor() time.Duration {
	if l == nil {
		return 0
	}
	return time.Since(l.acquired)
}

// WithLock ejecuta fn solo si se obtiene el lock no-bloqueante.
// Si está ocupado, fn NO se ejecuta y retorna (false, nil).
func (m *Manager) WithLock(ctx context.Context, keyName string, fn func(context.Context) error) (executed bool, err error) {
	lk, ok, err := m.TryAcquire(ctx, keyName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	defer func() {
		if relErr := lk.Release(ctx); relErr != nil && err == nil {
			err = relErr
		}
	}()
	if err := fn(ctx); err != nil {
		return true, err
	}
	return true, nil
}

// pgx.Conn type assertion no-op para mantener pgx import alive.
var _ = pgx.ErrNoRows
