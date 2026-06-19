//go:build integration

package timeline_test

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
	"nunezlagos/domain/internal/service/observation"
	projsvc "nunezlagos/domain/internal/service/project"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
)

type fix struct {
	tl        *timelinesvc.Service
	obs       *observation.Service
	prompts   *promptsvc.Service
	orgID     uuid.UUID
	projectID uuid.UUID
	userID    uuid.UUID
}

func setupTimeline(t *testing.T) (*fix, func()) {
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
	org, owner, _ := seedOrgUser(ctx, pools.App, "Acme", "acme", "o@x.com", "O")
	proj, _ := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})

	f := &fix{
		tl:        &timelinesvc.Service{Pool: pools.App},
		obs:       &observation.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}},
		prompts:   &promptsvc.Service{Pool: pools.App, Audit: rec},
		orgID:     org.ID,
		projectID: proj.ID,
		userID:    owner.UserID,
	}
	return f, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestContext_Snapshot_EmptyProject(t *testing.T) {
	f, cleanup := setupTimeline(t)
	defer cleanup()
	snap, err := f.tl.Context(context.Background(), f.orgID, f.userID, f.projectID)
	require.NoError(t, err)
	require.Nil(t, snap.ActiveSession)
	require.Empty(t, snap.RecentSessions)
	require.Empty(t, snap.RecentObservations)
	require.Empty(t, snap.RecentPrompts)
}

func TestContext_Snapshot_PopulatedProject(t *testing.T) {
	f, cleanup := setupTimeline(t)
	defer cleanup()
	ctx := context.Background()

	// REQ-42.3: sessions dropeada — el snapshot ya no incluye sesiones.
	// Observations
	for i, content := range []string{"obs uno", "obs dos", "obs tres"} {
		_, _ = f.obs.Save(ctx, observation.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.projectID, Content: content,
		})
		_ = i
	}
	// Prompt
	_, _ = f.prompts.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "test", Body: "Eres un asistente", SetActive: true,
	})

	snap, err := f.tl.Context(ctx, f.orgID, f.userID, f.projectID)
	require.NoError(t, err)
	require.Nil(t, snap.ActiveSession, "REQ-42.3: sessions dropeada — sin active session")
	require.Empty(t, snap.RecentSessions)
	require.Len(t, snap.RecentObservations, 3)
	require.Len(t, snap.RecentPrompts, 1)
}

func TestTimeline_AnchorMidStream(t *testing.T) {
	f, cleanup := setupTimeline(t)
	defer cleanup()
	ctx := context.Background()

	// Crear 5 observations en secuencia
	var ids []uuid.UUID
	for _, c := range []string{"a", "b", "c", "d", "e"} {
		o, err := f.obs.Save(ctx, observation.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.projectID, Content: c,
		})
		require.NoError(t, err)
		ids = append(ids, o.ID)
		time.Sleep(20 * time.Millisecond) // ensure created_at distinct
	}

	// Anchor en la del medio (c)
	tl, err := f.tl.Timeline(ctx, f.orgID, ids[2], 2, 2)
	require.NoError(t, err)
	require.Len(t, tl, 5, "2 before + anchor + 2 after")
	// El anchor debe estar al medio
	require.Equal(t, ids[2], tl[2].ID)
	require.Equal(t, "a", tl[0].Preview)
	require.Equal(t, "e", tl[4].Preview)
}

func TestTimeline_AnchorAtStart(t *testing.T) {
	f, cleanup := setupTimeline(t)
	defer cleanup()
	ctx := context.Background()
	o1, _ := f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "first",
	})
	time.Sleep(20 * time.Millisecond)
	_, _ = f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "second",
	})
	tl, err := f.tl.Timeline(ctx, f.orgID, o1.ID, 3, 3)
	require.NoError(t, err)
	// Anchor primero, después 1 entrada
	require.Equal(t, o1.ID, tl[0].ID)
	require.Equal(t, 2, len(tl))
}

func TestTimeline_AnchorNotFound(t *testing.T) {
	f, cleanup := setupTimeline(t)
	defer cleanup()
	_, err := f.tl.Timeline(context.Background(), f.orgID, uuid.New(), 3, 3)
	require.ErrorIs(t, err, timelinesvc.ErrObservationNotFound)
}

// Sabotaje: cross-org anchor lookup retorna NotFound (no leak).
func TestSabotage_Timeline_CrossOrgBlocked(t *testing.T) {
	f, cleanup := setupTimeline(t)
	defer cleanup()
	ctx := context.Background()
	o, _ := f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "secret",
	})
	// Otro org id (random)
	otherOrg := uuid.New()
	_, err := f.tl.Timeline(ctx, otherOrg, o.ID, 1, 1)
	require.ErrorIs(t, err, timelinesvc.ErrObservationNotFound)
}
