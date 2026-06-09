//go:build integration

package task_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	tsvc "nunezlagos/domain/internal/service/task"
)

type fix struct {
	svc  *tsvc.Service
	huID uuid.UUID
}

func setupTask(t *testing.T) (*fix, func()) {
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

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	rec := &audit.PGRecorder{Pool: pools.Auth}
	svc := &tsvc.Service{Pool: pools.App, Audit: rec}

	var reqID, huID uuid.UUID
	err = pools.App.QueryRow(ctx,
		`INSERT INTO requirements (slug, title) VALUES ('REQ-task-test', 'Task Test REQ') RETURNING id`,
	).Scan(&reqID)
	require.NoError(t, err)

	err = pools.App.QueryRow(ctx,
		`INSERT INTO user_stories (req_id, slug, title) VALUES ($1, 'HU-task-test', 'Test HU') RETURNING id`,
		reqID,
	).Scan(&huID)
	require.NoError(t, err)

	return &fix{svc: svc, huID: huID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestCreateTasks_Batch(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	inputs := []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "Create migration"},
		{Section: "Backend", Description: "Implement store"},
		{Section: "Tests", Description: "Integration test"},
		{Section: "Cierre", Description: "Manual verification"},
	}
	tasks, err := f.svc.CreateTasks(ctx, f.huID, inputs)
	require.NoError(t, err)
	require.Len(t, tasks, 4)
	for _, tsk := range tasks {
		require.Equal(t, tsvc.StatusPending, tsk.Status)
	}
	require.Equal(t, 1, tasks[0].Position)
	require.Equal(t, 2, tasks[1].Position)
}

func TestCreateTasks_EmptyFails(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.CreateTasks(ctx, f.huID, nil)
	require.Error(t, err)
}

func TestCreateTasks_HUNotFound(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.CreateTasks(ctx, uuid.New(), []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "test"},
	})
	require.ErrorIs(t, err, tsvc.ErrHUNotFound)
}

func TestListTasks_Ordered(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Tests", Description: "Test B"},
		{Section: "Backend", Description: "Backend A"},
		{Section: "Backend", Description: "Backend B"},
	})

	tasks, err := f.svc.ListTasks(ctx, f.huID)
	require.NoError(t, err)
	require.Len(t, tasks, 3)
	require.Equal(t, "Backend", tasks[0].Section)
	require.Equal(t, "Backend", tasks[1].Section)
	require.Equal(t, "Tests", tasks[2].Section)
}

func TestUpdateTaskStatus_PendingToInProgress(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	tasks, _ := f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "Do work"},
	})

	updated, err := f.svc.UpdateTaskStatus(ctx, tasks[0].ID, tsvc.StatusInProgress, "")
	require.NoError(t, err)
	require.Equal(t, tsvc.StatusInProgress, updated.Status)
	require.NotNil(t, updated.StartedAt)
}

func TestUpdateTaskStatus_InProgressToCompleted(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	tasks, _ := f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "Do work"},
	})
	_, _ = f.svc.UpdateTaskStatus(ctx, tasks[0].ID, tsvc.StatusInProgress, "")

	updated, err := f.svc.UpdateTaskStatus(ctx, tasks[0].ID, tsvc.StatusCompleted, "jdoe")
	require.NoError(t, err)
	require.Equal(t, tsvc.StatusCompleted, updated.Status)
	require.NotNil(t, updated.CompletedAt)
	require.NotNil(t, updated.CompletedBy)
	require.Equal(t, "jdoe", *updated.CompletedBy)
}

func TestUpdateTaskStatus_InvalidTransition(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	tasks, _ := f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "Skip"},
	})

	_, err := f.svc.UpdateTaskStatus(ctx, tasks[0].ID, tsvc.StatusCompleted, "")
	require.ErrorIs(t, err, tsvc.ErrInvalidTransition)
}

func TestUpdateTaskStatus_InvalidStatus(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	tasks, _ := f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "Bad status"},
	})

	_, err := f.svc.UpdateTaskStatus(ctx, tasks[0].ID, "bogus", "")
	require.ErrorIs(t, err, tsvc.ErrInvalidStatus)
}

func TestGetTask_WithVerificationAndSabotage(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	tasks, _ := f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "Full cycle"},
	})
	taskID := tasks[0].ID

	// Complete it
	_, _ = f.svc.UpdateTaskStatus(ctx, taskID, tsvc.StatusInProgress, "")
	_, _ = f.svc.UpdateTaskStatus(ctx, taskID, tsvc.StatusCompleted, "me")

	// Verify
	v, err := f.svc.CreateVerification(ctx, taskID, "pass", "Suite green", "All tests passed", "tester")
	require.NoError(t, err)
	require.Equal(t, "pass", v.Result)
	require.NotNil(t, v.Evidence)

	// Sabotage
	_, err = f.svc.CreateSabotage(ctx, taskID, "Drop index", "Search fails", "Got error", true)
	require.NoError(t, err)

	// Get with joins
	got, err := f.svc.GetTask(ctx, taskID)
	require.NoError(t, err)
	require.NotNil(t, got.Verification)
	require.Equal(t, "pass", got.Verification.Result)
	require.Len(t, got.Sabotages, 1)
}

func TestGetTask_NotFound(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.GetTask(ctx, uuid.New())
	require.ErrorIs(t, err, tsvc.ErrNotFound)
}

func TestGetProgress(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	tasks, _ := f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "T1"},
		{Section: "Backend", Description: "T2"},
		{Section: "Tests", Description: "T3"},
		{Section: "Tests", Description: "T4"},
		{Section: "Cierre", Description: "T5"},
	})

	// Complete 3 out of 5
	for i := 0; i < 3; i++ {
		_, _ = f.svc.UpdateTaskStatus(ctx, tasks[i].ID, tsvc.StatusInProgress, "")
		_, _ = f.svc.UpdateTaskStatus(ctx, tasks[i].ID, tsvc.StatusCompleted, "")
	}

	p, err := f.svc.GetProgress(ctx, f.huID)
	require.NoError(t, err)
	require.Equal(t, 5, p.Total)
	require.Equal(t, 3, p.Completed)
	require.InDelta(t, 60.0, p.ProgressPct, 0.1)
}

func TestCreateVerification_NotCompletedFails(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	tasks, _ := f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "Pending task"},
	})

	_, err := f.svc.CreateVerification(ctx, tasks[0].ID, "pass", "", "", "")
	require.ErrorIs(t, err, tsvc.ErrNotCompleted)
}

func TestCreateVerification_NotFound(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.CreateVerification(ctx, uuid.New(), "pass", "", "", "")
	require.ErrorIs(t, err, tsvc.ErrNotFound)
}

func TestCreateSabotage_NotFound(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.CreateSabotage(ctx, uuid.New(), "action", "", "", true)
	require.ErrorIs(t, err, tsvc.ErrNotFound)
}

func TestSabotage_DropFKBreaksVerification(t *testing.T) {
	f, cleanup := setupTask(t)
	defer cleanup()
	ctx := context.Background()

	tasks, _ := f.svc.CreateTasks(ctx, f.huID, []tsvc.CreateTaskInput{
		{Section: "Backend", Description: "Drop FK test"},
	})
	taskID := tasks[0].ID
	_, _ = f.svc.UpdateTaskStatus(ctx, taskID, tsvc.StatusInProgress, "")
	_, _ = f.svc.UpdateTaskStatus(ctx, taskID, tsvc.StatusCompleted, "")

	// Sabotage: drop FK on verification_results
	s, err := f.svc.CreateSabotage(ctx, taskID,
		"DROP CONSTRAINT verification_results_task_id_fkey",
		"INSERT verification fails",
		"Got FK error as expected",
		true,
	)
	require.NoError(t, err)
	require.True(t, s.Restored)
}
