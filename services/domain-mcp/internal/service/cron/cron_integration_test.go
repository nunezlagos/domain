//go:build integration

package cron_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/cron"
)

func setupCronDB(t *testing.T) (*pgxpool.Pool, func()) {
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

func TestCronService_PickDue_PicksDueCron(t *testing.T) {
	pool, cleanup := setupCronDB(t)
	defer cleanup()
	ctx := context.Background()

	cronID := uuid.New()
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `
		INSERT INTO crons (id, slug, name, cron_expression, target_type, target_id, next_run_at)
		VALUES ($1, 'test-cron', 'Test Cron', '* * * * *', 'flow', $2, $3)`,
		cronID, uuid.New(), now.Add(-time.Minute))
	require.NoError(t, err)

	svc := &cron.Service{Pool: pool}
	due, err := svc.PickDue(ctx, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, cronID, due[0].ID)
	require.Equal(t, "flow", due[0].TargetType)
	require.NotNil(t, due[0].LastRunAt, "last_run_at should be set after pick")
	require.NotNil(t, due[0].NextRunAt, "next_run_at should be advanced after pick")
}

func TestCronService_PickDue_DoesNotPickFuture(t *testing.T) {
	pool, cleanup := setupCronDB(t)
	defer cleanup()
	ctx := context.Background()

	future := time.Now().UTC().Add(24 * time.Hour)
	_, err := pool.Exec(ctx, `
		INSERT INTO crons (id, slug, name, cron_expression, target_type, target_id, next_run_at)
		VALUES ($1, 'future-cron', 'Future Cron', '* * * * *', 'flow', $2, $3)`,
		uuid.New(), uuid.New(), future)
	require.NoError(t, err)

	svc := &cron.Service{Pool: pool}
	due, err := svc.PickDue(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, due, "future cron should not be picked")
}

func TestCronService_PickDue_SkipsDisabled(t *testing.T) {
	pool, cleanup := setupCronDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO crons (id, slug, name, cron_expression, target_type, target_id, next_run_at, enabled)
		VALUES ($1, 'disabled-cron', 'Disabled Cron', '* * * * *', 'flow', $2, $3, false)`,
		uuid.New(), uuid.New(), time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)

	svc := &cron.Service{Pool: pool}
	due, err := svc.PickDue(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, due, "disabled cron should not be picked")
}

func TestCronService_PickDue_DoesNotPickDeleted(t *testing.T) {
	pool, cleanup := setupCronDB(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `
		INSERT INTO crons (id, slug, name, cron_expression, target_type, target_id, next_run_at, deleted_at)
		VALUES ($1, 'deleted-cron', 'Deleted Cron', '* * * * *', 'flow', $2, $3, $4)`,
		uuid.New(), uuid.New(), now.Add(-time.Minute), now)
	require.NoError(t, err)

	svc := &cron.Service{Pool: pool}
	due, err := svc.PickDue(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, due, "soft-deleted cron should not be picked")
}

func TestCronService_PickDue_RespectsLimit(t *testing.T) {
	pool, cleanup := setupCronDB(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		_, err := pool.Exec(ctx, `
			INSERT INTO crons (id, slug, name, cron_expression, target_type, target_id, next_run_at)
			VALUES ($1, $2, $3, '* * * * *', 'flow', $4, $5)`,
			uuid.New(), "cron-"+uuid.New().String(), "Cron", uuid.New(), now.Add(-time.Hour))
		require.NoError(t, err)
	}

	svc := &cron.Service{Pool: pool}
	due, err := svc.PickDue(ctx, 2)
	require.NoError(t, err)
	require.Len(t, due, 2, "should respect limit=2")
}

func insertCron(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	cronID := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO crons (id, slug, name, cron_expression, target_type, target_id, next_run_at)
		VALUES ($1, $2, 'Cron', '* * * * *', 'flow', $3, NOW())`,
		cronID, "cron-"+uuid.New().String(), uuid.New())
	require.NoError(t, err)
	return cronID
}

func TestCronService_ExecutionHistory_Lifecycle(t *testing.T) {
	pool, cleanup := setupCronDB(t)
	defer cleanup()
	ctx := context.Background()

	cronID := insertCron(t, pool)
	svc := &cron.Service{Pool: pool}


	execID, skipped, err := svc.StartExecution(ctx, cronID, "flow")
	require.NoError(t, err)
	require.False(t, skipped)
	require.NoError(t, svc.FinishExecution(ctx, execID, nil))


	execID2, skipped, err := svc.StartExecution(ctx, cronID, "flow")
	require.NoError(t, err)
	require.False(t, skipped, "sin running activa no hay overlap")
	require.NoError(t, svc.FinishExecution(ctx, execID2, context.DeadlineExceeded))

	hist, err := svc.History(ctx, cronID, 0)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, "failed", hist[0].Status, "más reciente primero")
	require.Contains(t, hist[0].Error, "deadline")
	require.Equal(t, "completed", hist[1].Status)
	require.NotNil(t, hist[1].FinishedAt)
	require.NotNil(t, hist[1].DurationMS)
}

// Sabotaje overlap: con una ejecución running, el siguiente Start NO dispara
// y deja rastro skipped_overlap en el historial.
func TestSabotage_CronService_OverlapSkipped(t *testing.T) {
	pool, cleanup := setupCronDB(t)
	defer cleanup()
	ctx := context.Background()

	cronID := insertCron(t, pool)
	svc := &cron.Service{Pool: pool}


	_, skipped, err := svc.StartExecution(ctx, cronID, "flow")
	require.NoError(t, err)
	require.False(t, skipped)


	_, skipped, err = svc.StartExecution(ctx, cronID, "flow")
	require.NoError(t, err)
	require.True(t, skipped, "previous running debe forzar skip")

	hist, err := svc.History(ctx, cronID, 0)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, "skipped_overlap", hist[0].Status)
	require.Equal(t, "running", hist[1].Status)


	otherID := insertCron(t, pool)
	_, skipped, err = svc.StartExecution(ctx, otherID, "flow")
	require.NoError(t, err)
	require.False(t, skipped, "el overlap es per-cron, no global")
}
