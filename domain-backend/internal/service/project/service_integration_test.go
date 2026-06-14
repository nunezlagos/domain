//go:build integration

package project_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	dmigrate "nunezlagos/domain/internal/migrate"
	orgsvc "nunezlagos/domain/internal/service/org"
	projsvc "nunezlagos/domain/internal/service/project"
)

func setupProj(t *testing.T) (*projsvc.Service, uuid.UUID, uuid.UUID, func()) {
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

	rec := &audit.PGRecorder{Pool: pool}
	orgS := &orgsvc.Service{Pool: pool, Audit: rec}
	org, owner, err := orgS.Create(ctx, "Acme", "acme", "o@x.com", "O")
	require.NoError(t, err)

	svc := &projsvc.Service{Pool: pool, Audit: rec}
	return svc, org.ID, owner.UserID, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestProject_Create(t *testing.T) {
	svc, orgID, owner, cleanup := setupProj(t)
	defer cleanup()
	ctx := context.Background()
	p, err := svc.Create(ctx, projsvc.CreateInput{
		OrganizationID: orgID,
		Name:           "Demo",
		Slug:           "demo",
		Description:    "desc",
		ActorID:        owner,
	})
	require.NoError(t, err)
	require.Equal(t, "demo", p.Slug)
	require.Equal(t, "Demo", p.Name)
}

func TestProject_SlugInvalid(t *testing.T) {
	svc, orgID, owner, cleanup := setupProj(t)
	defer cleanup()
	ctx := context.Background()
	for _, s := range []string{"UPPER", "with space", "_x", "x_", ""} {
		_, err := svc.Create(ctx, projsvc.CreateInput{
			OrganizationID: orgID, Name: "X", Slug: s, ActorID: owner,
		})
		require.ErrorIs(t, err, projsvc.ErrSlugInvalid, "slug %q", s)
	}
}

func TestProject_SlugTakenPerOrg(t *testing.T) {
	svc, orgID, owner, cleanup := setupProj(t)
	defer cleanup()
	ctx := context.Background()
	_, err := svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgID, Name: "A", Slug: "x", ActorID: owner})
	require.NoError(t, err)
	_, err = svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgID, Name: "B", Slug: "x", ActorID: owner})
	require.ErrorIs(t, err, projsvc.ErrSlugTaken)
}

func TestProject_GetBySlug(t *testing.T) {
	svc, orgID, owner, cleanup := setupProj(t)
	defer cleanup()
	ctx := context.Background()
	created, _ := svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgID, Name: "A", Slug: "alpha", ActorID: owner})
	got, err := svc.GetBySlug(ctx, orgID, "alpha")
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

func TestProject_List(t *testing.T) {
	svc, orgID, owner, cleanup := setupProj(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgID, Name: "A", Slug: "a", ActorID: owner})
	_, _ = svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgID, Name: "B", Slug: "b", ActorID: owner})
	list, err := svc.List(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestProject_Update(t *testing.T) {
	svc, orgID, owner, cleanup := setupProj(t)
	defer cleanup()
	ctx := context.Background()
	p, _ := svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgID, Name: "A", Slug: "a", ActorID: owner})
	newName := "Renamed"
	got, err := svc.Update(ctx, p.ID, projsvc.UpdateInput{Name: &newName, ActorID: owner})
	require.NoError(t, err)
	require.Equal(t, "Renamed", got.Name)
}

func TestProject_SoftDelete(t *testing.T) {
	svc, orgID, owner, cleanup := setupProj(t)
	defer cleanup()
	ctx := context.Background()
	p, _ := svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgID, Name: "A", Slug: "a", ActorID: owner})
	require.NoError(t, svc.SoftDelete(ctx, p.ID, owner))
	_, err := svc.GetBySlug(ctx, orgID, "a")
	require.ErrorIs(t, err, projsvc.ErrNotFound)
	require.NoError(t, svc.SoftDelete(ctx, p.ID, owner), "idempotente")
}

// Sabotaje: slug puede repetirse en DIFERENTES orgs.
func TestSabotage_Project_SlugReusableAcrossOrgs(t *testing.T) {
	svc, orgID, owner, cleanup := setupProj(t)
	defer cleanup()
	ctx := context.Background()

	// Crear segunda org en el mismo pool
	var orgB uuid.UUID
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('B', 'org-b') RETURNING id`).Scan(&orgB))

	_, err := svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgID, Name: "P", Slug: "shared", ActorID: owner})
	require.NoError(t, err)
	_, err = svc.Create(ctx, projsvc.CreateInput{OrganizationID: orgB, Name: "P", Slug: "shared", ActorID: owner})
	require.NoError(t, err, "mismo slug en otra org DEBE permitirse")
}
