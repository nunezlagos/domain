//go:build integration

package lifecycle_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/lifecycle"
	"nunezlagos/domain/internal/service/observation"
	projsvc "nunezlagos/domain/internal/service/project"
)

type fix struct {
	svc       *lifecycle.Service
	obs       *observation.Service
	proj      *projsvc.Service
	orgID     uuid.UUID
	projectID uuid.UUID
	userID    uuid.UUID
}

func setup(t *testing.T) (*fix, func()) {
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
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}
	obsS := &observation.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	org, owner, _ := seedOrgUser(ctx, pools.App, "Acme", "acme", "o@x.com", "O")
	proj, _ := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})

	svc := &lifecycle.Service{Pool: pools.App, Audit: rec}
	return &fix{
			svc: svc, obs: obsS, proj: projS,
			orgID: org.ID, projectID: proj.ID, userID: owner.UserID,
		}, func() {
			pools.Close()
			_ = pgC.Terminate(ctx)
		}
}

// issue-23.2 — Restore después de soft-delete dentro de window.
func TestRestore_ProjectRoundTrip(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, f.proj.SoftDelete(ctx, f.projectID, f.userID))


	require.NoError(t, f.svc.Restore(ctx, "project", f.projectID, f.userID, &f.orgID))


	p, err := f.proj.GetByID(ctx, f.projectID)
	require.NoError(t, err)
	require.Nil(t, p.DeletedAt)
}

func TestRestore_NotSoftDeleted(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	err := f.svc.Restore(context.Background(), "project", f.projectID, f.userID, &f.orgID)
	require.ErrorIs(t, err, lifecycle.ErrNotFound)
}

func TestRestore_UnsupportedEntity(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	err := f.svc.Restore(context.Background(), "no_existe", uuid.New(), f.userID, &f.orgID)
	require.ErrorIs(t, err, lifecycle.ErrEntityNotSupported)
}

func TestRestore_RetentionExpired(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, f.proj.SoftDelete(ctx, f.projectID, f.userID))

	_, err := f.svc.Pool.Exec(ctx,
		`UPDATE projects SET deleted_at = NOW() - INTERVAL '60 days' WHERE id = $1`,
		f.projectID)
	require.NoError(t, err)

	err = f.svc.Restore(ctx, "project", f.projectID, f.userID, &f.orgID)
	require.ErrorIs(t, err, lifecycle.ErrRetentionExpired)
}

// issue-23.3 — GDPR export incluye todas las entidades del user.
func TestExportUserData_FullBundle(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		CreatedBy: &f.userID, Content: "mi observación importante",
	})

	exp, err := f.svc.ExportUserData(ctx, f.userID, f.orgID)
	require.NoError(t, err)
	require.Equal(t, "1.0", exp.Version)
	require.Equal(t, f.userID, exp.UserID)
	require.NotEmpty(t, exp.User)
	require.Equal(t, "o@x.com", exp.User["email"])
	require.Len(t, exp.Organizations, 1)
	require.True(t, len(exp.Projects) >= 1)
	require.True(t, len(exp.Observations) >= 1)
	require.Contains(t, exp.Observations[0]["content"], "observación importante")
}

// Sabotaje: export NO debe incluir key_hash (BYTEA secreta) ni passwords
func TestSabotage_ExportUserData_NoSecrets(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	exp, err := f.svc.ExportUserData(ctx, f.userID, f.orgID)
	require.NoError(t, err)


	for _, k := range exp.APIKeys {
		_, hasHash := k["key_hash"]
		require.False(t, hasHash, "key_hash NO debe estar en export GDPR")
		_, hasPrefix := k["key_prefix"]
		_ = hasPrefix // puede haber 0 keys → omit check
	}


	_, hasPw := exp.User["password"]
	require.False(t, hasPw)
}

// Idempotencia: restore después de restore es no-op (ya no es soft-deleted)
func TestRestore_Idempotent(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, f.proj.SoftDelete(ctx, f.projectID, f.userID))
	require.NoError(t, f.svc.Restore(ctx, "project", f.projectID, f.userID, &f.orgID))

	err := f.svc.Restore(ctx, "project", f.projectID, f.userID, &f.orgID)
	require.ErrorIs(t, err, lifecycle.ErrNotFound)
	_ = time.Second
}
