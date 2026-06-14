//go:build integration

package session_test

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
	orgsvc "nunezlagos/domain/internal/service/org"
	projsvc "nunezlagos/domain/internal/service/project"
	sesssvc "nunezlagos/domain/internal/service/session"
)

type fix struct {
	svc       *sesssvc.Service
	orgID     uuid.UUID
	userID    uuid.UUID
	projectID uuid.UUID
}

func setupSession(t *testing.T) (*fix, func()) {
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
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}
	org, owner, _ := orgS.Create(ctx, "Acme", "acme", "o@x.com", "O")
	proj, _ := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})

	svc := &sesssvc.Service{Pool: pools.App, Audit: rec}
	return &fix{svc: svc, orgID: org.ID, userID: owner.UserID, projectID: proj.ID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestSession_StartAndStatus(t *testing.T) {
	f, cleanup := setupSession(t)
	defer cleanup()
	ctx := context.Background()
	sess, err := f.svc.Start(ctx, sesssvc.StartInput{
		OrganizationID: f.orgID,
		UserID:         f.userID,
		ProjectID:      &f.projectID,
		Title:          "trabajo de hoy",
	})
	require.NoError(t, err)
	require.Equal(t, "active", sess.Status())
	require.Nil(t, sess.EndedAt)
	require.NotEmpty(t, sess.StartedAt)
}

func TestSession_Start_TitleRequired(t *testing.T) {
	f, cleanup := setupSession(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Start(ctx, sesssvc.StartInput{
		OrganizationID: f.orgID, UserID: f.userID, Title: "   ",
	})
	require.ErrorIs(t, err, sesssvc.ErrTitleRequired)
}

func TestSession_GetActive(t *testing.T) {
	f, cleanup := setupSession(t)
	defer cleanup()
	ctx := context.Background()
	sess, _ := f.svc.Start(ctx, sesssvc.StartInput{
		OrganizationID: f.orgID, UserID: f.userID, ProjectID: &f.projectID, Title: "A",
	})
	active, err := f.svc.GetActive(ctx, f.userID, f.projectID)
	require.NoError(t, err)
	require.Equal(t, sess.ID, active.ID)
}

func TestSession_End(t *testing.T) {
	f, cleanup := setupSession(t)
	defer cleanup()
	ctx := context.Background()
	sess, _ := f.svc.Start(ctx, sesssvc.StartInput{
		OrganizationID: f.orgID, UserID: f.userID, ProjectID: &f.projectID, Title: "T",
	})
	ended, err := f.svc.End(ctx, sess.ID, f.userID, "Se completaron 3 tareas")
	require.NoError(t, err)
	require.NotNil(t, ended.EndedAt)
	require.Equal(t, "Se completaron 3 tareas", ended.Summary)
	require.Equal(t, "completed", ended.Status())

	// GetActive ya no la devuelve
	_, err = f.svc.GetActive(ctx, f.userID, f.projectID)
	require.ErrorIs(t, err, sesssvc.ErrNotFound)
}

func TestSession_EndTwice_Rejected(t *testing.T) {
	f, cleanup := setupSession(t)
	defer cleanup()
	ctx := context.Background()
	sess, _ := f.svc.Start(ctx, sesssvc.StartInput{
		OrganizationID: f.orgID, UserID: f.userID, ProjectID: &f.projectID, Title: "T",
	})
	_, err := f.svc.End(ctx, sess.ID, f.userID, "x")
	require.NoError(t, err)
	_, err = f.svc.End(ctx, sess.ID, f.userID, "y")
	require.ErrorIs(t, err, sesssvc.ErrAlreadyEnded)
}

func TestSession_List_RecentFirst(t *testing.T) {
	f, cleanup := setupSession(t)
	defer cleanup()
	ctx := context.Background()
	a, _ := f.svc.Start(ctx, sesssvc.StartInput{
		OrganizationID: f.orgID, UserID: f.userID, ProjectID: &f.projectID, Title: "A",
	})
	b, _ := f.svc.Start(ctx, sesssvc.StartInput{
		OrganizationID: f.orgID, UserID: f.userID, ProjectID: &f.projectID, Title: "B",
	})
	list, err := f.svc.List(ctx, f.userID, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
	// b creada después → primero en orden DESC por started_at
	require.True(t, list[0].StartedAt.Equal(b.StartedAt) || list[0].StartedAt.After(a.StartedAt))
}

// Sabotaje: GetActive con project filter no devuelve sesiones de otros projects.
func TestSabotage_GetActive_ProjectFiltered(t *testing.T) {
	f, cleanup := setupSession(t)
	defer cleanup()
	ctx := context.Background()

	// Crear segundo project + session en él
	rec := &audit.PGRecorder{Pool: f.svc.Pool}
	projS := &projsvc.Service{Pool: f.svc.Pool, Audit: rec}
	proj2, err := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: f.orgID, Name: "P2", Slug: "p2", ActorID: f.userID,
	})
	require.NoError(t, err)
	_, _ = f.svc.Start(ctx, sesssvc.StartInput{
		OrganizationID: f.orgID, UserID: f.userID, ProjectID: &proj2.ID, Title: "other",
	})

	// GetActive sobre el primer project: no debe encontrar nada
	_, err = f.svc.GetActive(ctx, f.userID, f.projectID)
	require.ErrorIs(t, err, sesssvc.ErrNotFound,
		"sesiones de otro project NO deben aparecer")
}
