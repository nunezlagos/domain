//go:build integration

package search_test

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
	searchsvc "nunezlagos/domain/internal/service/search"
)

type fix struct {
	search    *searchsvc.Service
	obs       *observation.Service
	prompts   *promptsvc.Service
	orgID     uuid.UUID
	projectID uuid.UUID
	userID    uuid.UUID
}

func setupSearch(t *testing.T) (*fix, func()) {
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

	return &fix{
			search:  &searchsvc.Service{Pool: pools.App},
			obs:     &observation.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}},
			prompts: &promptsvc.Service{Pool: pools.App, Audit: rec},
			orgID:   org.ID, projectID: proj.ID, userID: owner.UserID,
		}, func() {
			pools.Close()
			_ = pgC.Terminate(ctx)
		}
}

func TestSearch_AllEntities(t *testing.T) {
	f, cleanup := setupSearch(t)
	defer cleanup()
	ctx := context.Background()



	_, _ = f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "Decidimos usar pgvector para embeddings",
	})
	_, _ = f.prompts.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "embed", Body: "Genera embedding con pgvector ivfflat", SetActive: true,
	})

	results, err := f.search.Search(ctx, f.orgID, "pgvector", 20, searchsvc.Filter{})
	require.NoError(t, err)
	require.NotEmpty(t, results)


	seen := map[searchsvc.EntityType]bool{}
	for _, r := range results {
		seen[r.EntityType] = true
	}
	require.True(t, seen[searchsvc.EntityObservation], "obs presente")
	require.True(t, seen[searchsvc.EntityPrompt], "prompt presente")
	require.False(t, seen[searchsvc.EntitySession], "REQ-42.3: sessions dropeada")
}

func TestSearch_EmptyQuery_Error(t *testing.T) {
	f, cleanup := setupSearch(t)
	defer cleanup()
	_, err := f.search.Search(context.Background(), f.orgID, "  ", 10, searchsvc.Filter{})
	require.ErrorIs(t, err, searchsvc.ErrEmptyQuery)
}

func TestSearch_FilterEntityType(t *testing.T) {
	f, cleanup := setupSearch(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "topic xyz",
	})
	_, _ = f.prompts.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "p", Body: "topic xyz", SetActive: true,
	})

	results, err := f.search.Search(ctx, f.orgID, "topic", 10,
		searchsvc.Filter{EntityTypes: []searchsvc.EntityType{searchsvc.EntityPrompt}})
	require.NoError(t, err)
	for _, r := range results {
		require.Equal(t, searchsvc.EntityPrompt, r.EntityType)
	}
}

func TestSearch_FilterByTags(t *testing.T) {
	f, cleanup := setupSearch(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "alpha note", Tags: []string{"production"},
	})
	_, _ = f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "alpha note dev", Tags: []string{"dev"},
	})
	results, err := f.search.Search(ctx, f.orgID, "alpha", 10,
		searchsvc.Filter{Tags: []string{"production"}})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	for _, r := range results {
		if r.EntityType == searchsvc.EntityObservation {
			require.Contains(t, r.Tags, "production")
		}
	}
}

func TestSearch_FilterDateRange(t *testing.T) {
	f, cleanup := setupSearch(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.obs.Save(ctx, observation.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "alpha entry",
	})
	future := time.Now().Add(24 * time.Hour)
	results, err := f.search.Search(ctx, f.orgID, "alpha", 10,
		searchsvc.Filter{DateFrom: &future})
	require.NoError(t, err)
	require.Empty(t, results, "DateFrom futuro debe excluir todo")
}
