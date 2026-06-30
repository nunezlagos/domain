//go:build integration

package leader_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/scheduler/leader"
)

func setup(t *testing.T) (*pgxpool.Pool, func()) {
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
	require.NoError(t, dmigrate.Up(dsn))
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

const testLockKey = 99999

func TestLeader_FirstWinsLockSecondLoses(t *testing.T) {
	pool, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	e1 := &leader.Election{Pool: pool, LockKey: testLockKey}
	e2 := &leader.Election{Pool: pool, LockKey: testLockKey}

	got1, err := e1.TryAcquire(ctx)
	require.NoError(t, err)
	require.True(t, got1, "primer pod debe ganar")

	got2, err := e2.TryAcquire(ctx)
	require.NoError(t, err)
	require.False(t, got2, "segundo pod NO debe ganar mientras e1 tiene lock")

	e1.Release(ctx)

	got2, err = e2.TryAcquire(ctx)
	require.NoError(t, err)
	require.True(t, got2, "tras release del primer pod, segundo gana")
	e2.Release(ctx)
}

func TestLeader_RunAsLeader_ExecutesWorkAndReleases(t *testing.T) {
	pool, cleanup := setup(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	e := &leader.Election{Pool: pool, LockKey: testLockKey + 1, PollPeriod: 50 * time.Millisecond}
	var executed atomic.Bool
	done := make(chan struct{})

	go e.RunAsLeader(ctx, func(leaderCtx context.Context) {
		executed.Store(true)
		select {
		case <-leaderCtx.Done():
		case <-time.After(100 * time.Millisecond):
		}
		close(done)
	})
	<-done
	require.True(t, executed.Load())


	e2 := &leader.Election{Pool: pool, LockKey: testLockKey + 1}
	got, _ := e2.TryAcquire(ctx)
	require.True(t, got, "tras RunAsLeader, lock liberado y otro puede tomar")
	e2.Release(ctx)
}

func TestLeader_DifferentKeysDontConflict(t *testing.T) {
	pool, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	e1 := &leader.Election{Pool: pool, LockKey: testLockKey + 100}
	e2 := &leader.Election{Pool: pool, LockKey: testLockKey + 101}

	got1, _ := e1.TryAcquire(ctx)
	got2, _ := e2.TryAcquire(ctx)
	require.True(t, got1)
	require.True(t, got2, "keys distintos NO compiten")
	e1.Release(ctx)
	e2.Release(ctx)
}

// Sabotaje: TryAcquire dos veces seguidas desde el mismo pod debe ser idempotente
// (ya tenemos lock, devuelve true sin tomar de nuevo)
func TestSabotage_Leader_TryAcquireIdempotent(t *testing.T) {
	pool, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	e := &leader.Election{Pool: pool, LockKey: testLockKey + 200}
	for i := 0; i < 5; i++ {
		got, err := e.TryAcquire(ctx)
		require.NoError(t, err)
		require.True(t, got, "TryAcquire desde el mismo Election es idempotente")
	}
	e.Release(ctx)
}
