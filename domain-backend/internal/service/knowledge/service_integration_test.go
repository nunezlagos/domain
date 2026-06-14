//go:build integration

package knowledge_test

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
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/knowledge"
	orgsvc "nunezlagos/domain/internal/service/org"
	projsvc "nunezlagos/domain/internal/service/project"
)

type fix struct {
	svc       *knowledge.Service
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
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}
	org, owner, _ := orgS.Create(ctx, "Acme", "acme", "o@x.com", "O")
	proj, _ := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})

	svc := &knowledge.Service{
		Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{},
	}
	return &fix{svc: svc, orgID: org.ID, projectID: proj.ID, userID: owner.UserID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestKnowledge_Save_ShortDocSingleChunk(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	doc, chunks, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, CreatedBy: &f.userID,
		Title: "Arquitectura del sistema",
		Body:  "Domain usa pgvector + tsvector para búsqueda híbrida.",
	})
	require.NoError(t, err)
	require.Equal(t, "Arquitectura del sistema", doc.Title)
	require.Len(t, chunks, 1)
	require.Equal(t, 0, chunks[0].ChunkIndex)
}

func TestKnowledge_Save_LongDocMultipleChunks(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	body := strings.Repeat("Este es un párrafo importante.\n\n", 200)
	doc, chunks, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "long doc", Body: body,
	})
	require.NoError(t, err)
	require.True(t, len(chunks) >= 3, "doc largo debe chunkearse en múltiples")
	for i, c := range chunks {
		require.Equal(t, i, c.ChunkIndex, "chunk_index secuencial")
		require.Equal(t, doc.ID, c.DocumentID)
	}
}

func TestKnowledge_Save_TitleRequired(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Title: "  ", Body: "x",
	})
	require.ErrorIs(t, err, knowledge.ErrTitleRequired)
}

func TestKnowledge_Get_ReturnsDocPlusChunks(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	doc, _, _ := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "T", Body: strings.Repeat("hola amigo. ", 300),
	})
	got, chunks, err := f.svc.Get(ctx, doc.ID)
	require.NoError(t, err)
	require.Equal(t, doc.ID, got.ID)
	require.True(t, len(chunks) >= 1)
}

func TestKnowledge_SearchHybrid_Semantic(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "A", Body: "El sistema usa pgvector para búsqueda semántica con cosine.",
	})
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "B", Body: "El clima ayer fue lluvioso en Santiago.",
	})

	results, err := f.svc.SearchHybrid(ctx, f.orgID, "pgvector búsqueda", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Contains(t, strings.ToLower(results[0].Snippet), "pgvector")
}

func TestKnowledge_SearchHybrid_NopEmbedderDegradesToTSVector(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	f.svc.Embedder = llm.NopEmbedder{}
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "T", Body: "documentación del módulo de memoria de domain.",
	})
	results, err := f.svc.SearchHybrid(ctx, f.orgID, "memoria domain", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
}

func TestKnowledge_ListByProject(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Title: "A", Body: "a",
	})
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Title: "B", Body: "b",
	})
	list, err := f.svc.ListByProject(ctx, f.projectID, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestKnowledge_SoftDelete(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	doc, _, _ := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Title: "T", Body: "y",
	})
	require.NoError(t, f.svc.SoftDelete(ctx, doc.ID, f.userID))
	_, _, err := f.svc.Get(ctx, doc.ID)
	require.ErrorIs(t, err, knowledge.ErrNotFound)
}

// Sabotaje: cross-org search no leak
func TestSabotage_Knowledge_OrgScoped(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "secreto", Body: "información confidencial de la org A",
	})

	results, err := f.svc.SearchHybrid(ctx, uuid.New(), "confidencial", 10)
	require.NoError(t, err)
	require.Empty(t, results)
}

// Sabotaje: si Embed falla, Save NO crea doc parcial (atómico)
type failingEmbedder struct{ dim int }

func (failingEmbedder) Dimensions() int { return 1536 }
func (failingEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, errors.New("synthetic embedder failure")
}

var errors = struct{ New func(string) error }{New: func(s string) error { return embErr(s) }}

type embErr string

func (e embErr) Error() string { return string(e) }

func TestSabotage_Knowledge_EmbedFailureAtomic(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	f.svc.Embedder = failingEmbedder{}
	_, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "T", Body: "body que va a fallar embedding",
	})
	require.Error(t, err)

	var count int
	require.NoError(t, f.svc.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM knowledge_docs WHERE title = 'T'`).Scan(&count))
	require.Equal(t, 0, count, "doc NO debe persistir si embed falla")
}
