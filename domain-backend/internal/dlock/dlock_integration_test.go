//go:build integration

package dlock

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/metrics"
)

func setupPool(t *testing.T) (*pgxpool.Pool, string, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return pool, dsn, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// test-001: N goroutines compiten por el mismo key — exactamente una gana.
func TestTryAcquire_Concurrent_OnlyOneWins(t *testing.T) {
	pool, _, cleanup := setupPool(t)
	defer cleanup()
	ctx := context.Background()
	m := &Manager{Pool: pool, Metrics: metrics.New()}

	const workers = 8
	var acquired atomic.Int32
	var wg sync.WaitGroup
	locks := make([]*Lock, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			lk, ok, err := m.TryAcquire(ctx, "contended-key")
			require.NoError(t, err)
			if ok {
				acquired.Add(1)
				locks[idx] = lk
			}
		}(i)
	}
	wg.Wait()
	require.Equal(t, int32(1), acquired.Load(), "solo un worker debe ganar el lock")

	for _, lk := range locks {
		if lk != nil {
			require.NoError(t, lk.Release(ctx))
		}
	}

	// Liberado → otro acquire gana
	lk, ok, err := m.TryAcquire(ctx, "contended-key")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, lk.Release(ctx))
}

// test-002: si la session muere, Postgres libera el lock solo. Se simula el
// crash terminando el backend del holder con pg_terminate_backend.
func TestConnDie_AutoReleases(t *testing.T) {
	pool, dsn, cleanup := setupPool(t)
	defer cleanup()
	ctx := context.Background()

	// Pool secundario que toma el lock y "muere"
	dying, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	mDying := &Manager{Pool: dying}
	_, ok, err := mDying.TryAcquire(ctx, "auto-release-key")
	require.NoError(t, err)
	require.True(t, ok)

	// Mientras vive, el lock está ocupado
	m := &Manager{Pool: pool}
	_, ok, err = m.TryAcquire(ctx, "auto-release-key")
	require.NoError(t, err)
	require.False(t, ok, "lock debe estar ocupado mientras la otra session vive")

	// Crash simulado: terminar el backend que sostiene el advisory lock
	var pid int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pid FROM pg_locks WHERE locktype = 'advisory' LIMIT 1`).Scan(&pid))
	_, err = pool.Exec(ctx, `SELECT pg_terminate_backend($1)`, pid)
	require.NoError(t, err)

	// Postgres libera advisory locks al morir la session
	deadline := time.Now().Add(5 * time.Second)
	for {
		lk, ok, err := m.TryAcquire(ctx, "auto-release-key")
		require.NoError(t, err)
		if ok {
			require.NoError(t, lk.Release(ctx))
			break
		}
		require.True(t, time.Now().Before(deadline), "lock nunca se liberó tras morir la conn")
		time.Sleep(100 * time.Millisecond)
	}
	// Nota: no cerramos `dying` — su conn fue terminada server-side y Close()
	// bloquearía esperando un Release que nunca llega (comportamiento esperado
	// de pgxpool con conns acquired).
}

// test-003: Acquire con maxWait corto sobre key ocupada → ErrTimeout.
func TestAcquire_WaitTimeout(t *testing.T) {
	pool, _, cleanup := setupPool(t)
	defer cleanup()
	ctx := context.Background()
	m := &Manager{Pool: pool}

	holder, ok, err := m.TryAcquire(ctx, "timeout-key")
	require.NoError(t, err)
	require.True(t, ok)
	defer holder.Release(ctx)

	start := time.Now()
	_, err = m.Acquire(ctx, "timeout-key", 600*time.Millisecond)
	require.ErrorIs(t, err, ErrTimeout)
	require.GreaterOrEqual(t, time.Since(start), 600*time.Millisecond)
}

// dl-005: métricas registran acquired/busy y held duration.
func TestMetrics_AcquireAndHeld(t *testing.T) {
	pool, _, cleanup := setupPool(t)
	defer cleanup()
	ctx := context.Background()
	reg := metrics.New()
	m := &Manager{Pool: pool, Metrics: reg}

	lk, ok, err := m.TryAcquire(ctx, "metrics-key")
	require.NoError(t, err)
	require.True(t, ok)

	_, busy, err := m.TryAcquire(ctx, "metrics-key")
	require.NoError(t, err)
	require.False(t, busy)

	require.NoError(t, lk.Release(ctx))

	families, err := reg.Prometheus().Gather()
	require.NoError(t, err)
	found := map[string]bool{}
	for _, mf := range families {
		switch mf.GetName() {
		case "domain_dlock_acquire_total":
			for _, metric := range mf.GetMetric() {
				for _, l := range metric.GetLabel() {
					if l.GetName() == "result" {
						found[l.GetValue()] = true
					}
				}
			}
		case "domain_dlock_held_duration_seconds":
			found["held"] = true
		}
	}
	require.True(t, found["acquired"], "falta counter result=acquired")
	require.True(t, found["busy"], "falta counter result=busy")
	require.True(t, found["held"], "falta histogram held_duration")
}
