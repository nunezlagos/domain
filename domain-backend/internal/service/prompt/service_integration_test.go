//go:build integration

package prompt_test

import (
	"context"
	"strings"
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
	promptsvc "nunezlagos/domain/internal/service/prompt"
)

type fix struct {
	svc       *promptsvc.Service
	orgID     uuid.UUID
	projectID uuid.UUID
	userID    uuid.UUID
}

func setupPrompt(t *testing.T) (*fix, func()) {
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

	svc := &promptsvc.Service{Pool: pools.App, Audit: rec}
	return &fix{svc: svc, orgID: org.ID, projectID: proj.ID, userID: owner.UserID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestPrompt_Create_FirstVersionIs1(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	p, err := f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID, CreatedBy: &f.userID,
		Slug: "greeting", Body: "Hola, {{name}}",
		Variables: []promptsvc.Variable{{Name: "name", Type: "string", Required: true}},
		SetActive: true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, p.Version)
	require.True(t, p.IsActive)
	require.Len(t, p.Variables, 1)
}

func TestPrompt_Create_SecondVersionIncrements(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "g", Body: "v1", SetActive: true,
	})
	p2, err := f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "g", Body: "v2", SetActive: true,
	})
	require.NoError(t, err)
	require.Equal(t, 2, p2.Version)
	require.True(t, p2.IsActive)

	// v1 quedó inactive
	versions, err := f.svc.ListVersions(ctx, f.orgID, &f.projectID, "g")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	require.Equal(t, 2, versions[0].Version)
	require.False(t, versions[1].IsActive, "v1 debe quedar inactive")
}

func TestPrompt_Create_InvalidSlug(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "UPPER", Body: "x",
	})
	require.ErrorIs(t, err, promptsvc.ErrSlugInvalid)
}

func TestPrompt_Create_EmptyBody(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "x", Body: "   ",
	})
	require.ErrorIs(t, err, promptsvc.ErrBodyRequired)
}

func TestPrompt_GetActive(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	v1, _ := f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "a", Body: "v1", SetActive: true,
	})
	got, err := f.svc.GetActive(ctx, f.orgID, &f.projectID, "a")
	require.NoError(t, err)
	require.Equal(t, v1.ID, got.ID)
}

func TestPrompt_SetActive_PromotesPreviousVersion(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	v1, _ := f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "a", Body: "v1", SetActive: true,
	})
	_, _ = f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "a", Body: "v2", SetActive: true,
	})
	// Volver a activar v1
	_, err := f.svc.SetActive(ctx, v1.ID, f.userID)
	require.NoError(t, err)
	active, err := f.svc.GetActive(ctx, f.orgID, &f.projectID, "a")
	require.NoError(t, err)
	require.Equal(t, v1.ID, active.ID)
	require.Equal(t, 1, active.Version)
}

func TestPrompt_Search_Headline(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "code-review", Body: "Hace una revisión del código en busca de bugs",
		SetActive: true,
	})
	_, _ = f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "greeting", Body: "Hola amigo cómo estás",
		SetActive: true,
	})
	results, err := f.svc.Search(ctx, f.orgID, "código bugs", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Contains(t, strings.ToLower(results[0].Body), "código")
	require.Contains(t, results[0].Headline, "<mark>",
		"headline debe destacar matches con <mark>")
}

func TestPrompt_SoftDelete(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	p, _ := f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "x", Body: "y", SetActive: true,
	})
	require.NoError(t, f.svc.SoftDelete(ctx, p.ID, f.userID))
	_, err := f.svc.GetActive(ctx, f.orgID, &f.projectID, "x")
	require.ErrorIs(t, err, promptsvc.ErrNoActiveVersion)
}

// Sabotaje: 2 versiones activas simultáneas NO permitido (SetActive desactiva otras).
func TestSabotage_OnlyOneActiveVersion(t *testing.T) {
	f, cleanup := setupPrompt(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "z", Body: "v1", SetActive: true,
	})
	_, _ = f.svc.Create(ctx, promptsvc.CreateInput{
		OrganizationID: f.orgID, ProjectID: &f.projectID,
		Slug: "z", Body: "v2", SetActive: true,
	})

	var activeCount int
	require.NoError(t, f.svc.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM prompts WHERE slug = 'z' AND is_active = true AND deleted_at IS NULL`).Scan(&activeCount))
	require.Equal(t, 1, activeCount, "solo UNA versión activa a la vez por slug")
}
