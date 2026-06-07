//go:build integration

package observation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/saargo/domain/internal/audit"
	"github.com/saargo/domain/internal/llm"
	dmigrate "github.com/saargo/domain/internal/migrate"
	obssvc "github.com/saargo/domain/internal/service/observation"
	orgsvc "github.com/saargo/domain/internal/service/org"
	projsvc "github.com/saargo/domain/internal/service/project"
)

type fixture struct {
	svc       *obssvc.Service
	orgID     uuid.UUID
	projectID uuid.UUID
	owner     uuid.UUID
}

func setup(t *testing.T) (*fixture, func()) {
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
	projS := &projsvc.Service{Pool: pool, Audit: rec}

	org, owner, err := orgS.Create(ctx, "Acme", "acme", "o@x.com", "O")
	require.NoError(t, err)
	proj, err := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})
	require.NoError(t, err)

	svc := &obssvc.Service{Pool: pool, Audit: rec, Embedder: llm.FakeEmbedder{}}
	f := &fixture{svc: svc, orgID: org.ID, projectID: proj.ID, owner: owner.UserID}
	return f, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestObservation_Save(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	o, err := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID,
		ProjectID:      f.projectID,
		CreatedBy:      &f.owner,
		Content:        "decidimos usar pgvector en lugar de pinecone",
		Tags:           []string{"arch", "db"},
		Metadata:       map[string]any{"source": "design.md"},
	})
	require.NoError(t, err)
	require.Equal(t, "note", o.ObservationType)
	require.ElementsMatch(t, []string{"arch", "db"}, o.Tags)
	require.Equal(t, "design.md", o.Metadata["source"])
}

func TestObservation_Save_EmptyContent(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "   ",
	})
	require.ErrorIs(t, err, obssvc.ErrContentRequired)
}

func TestObservation_GetAndList(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	a, _ := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "uno",
	})
	_, _ = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "dos",
	})

	got, err := f.svc.Get(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "uno", got.Content)

	list, err := f.svc.List(ctx, f.projectID, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestObservation_SoftDelete(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	o, _ := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Content: "borrar",
	})
	require.NoError(t, f.svc.SoftDelete(ctx, o.ID, f.owner))
	_, err := f.svc.Get(ctx, o.ID)
	require.ErrorIs(t, err, obssvc.ErrNotFound)
}

// Búsqueda híbrida: FakeEmbedder con frase similar debe rankear alto.
func TestObservation_SearchHybrid_FindsBM25Match(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "decidimos usar pgvector con embeddings de OpenAI"})
	_, _ = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "el clima en santiago está soleado"})
	_, _ = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "pgvector permite búsqueda semántica con ivfflat"})

	results, err := f.svc.SearchHybrid(ctx, f.orgID, "pgvector embeddings", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.True(t, len(results) >= 2,
		"al menos 2 obs sobre pgvector deben matchear")
	// Top result debe ser sobre pgvector, no sobre clima
	require.Contains(t, results[0].Content, "pgvector")
}

// Búsqueda híbrida degrada a tsvector-only si embedder es Nop.
func TestObservation_SearchHybrid_NopEmbedder_TSVectorOnly(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	// Cambiamos embedder a Nop después del setup
	f.svc.Embedder = llm.NopEmbedder{}
	_, _ = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "documentación de domain mcp server"})
	_, _ = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "código tradicional sin tema relacionado"})

	results, err := f.svc.SearchHybrid(ctx, f.orgID, "domain mcp", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results, "tsvector-only debe encontrar la frase")
	require.Contains(t, results[0].Content, "domain")
}

// Empty query devuelve nil.
func TestObservation_SearchHybrid_EmptyQuery(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	results, err := f.svc.SearchHybrid(ctx, f.orgID, "   ", 10)
	require.NoError(t, err)
	require.Nil(t, results)
}

// HU-03.6: dedup detecta duplicado exacto.
func TestObservation_Save_DedupRejectsDuplicate(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "Fix login bug",
	})
	require.NoError(t, err)
	_, err = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "Fix login bug",
	})
	require.ErrorIs(t, err, obssvc.ErrDuplicate)
}

// HU-03.6: normalización detecta duplicado con whitespace/caso distinto.
func TestObservation_Save_DedupNormalizes(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "Fix Login",
	})
	require.NoError(t, err)
	_, err = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "fix  login",
	})
	require.ErrorIs(t, err, obssvc.ErrDuplicate,
		"variantes de caso/whitespace deben detectarse como duplicados")
}

// HU-03.6: distinto observation_type permite mismo content.
func TestObservation_Save_DedupScopedByType(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "Mismo contenido", ObservationType: "note",
	})
	require.NoError(t, err)
	_, err = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "Mismo contenido", ObservationType: "decision",
	})
	require.NoError(t, err, "distinto type → distinto hash → permitido")
}

// HU-03.6: privacy stripping remueve bloques <private>.
func TestObservation_Save_StripsPrivateBlocks(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	o, err := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "antes <private>SECRETO</private> después",
	})
	require.NoError(t, err)
	require.NotContains(t, o.Content, "SECRETO",
		"contenido privado NO debe persistir")
	require.NotContains(t, o.Content, "<private>")
	// metadata.privacy_redacted_blocks debe estar
	require.EqualValues(t, 1, o.Metadata["privacy_redacted_blocks"])
}

// Sabotaje DB-level: el UNIQUE PARTIAL atrapa duplicados aunque la app bypass.
func TestSabotage_Dedup_DBConstraintEnforces(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	o, err := f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "unico",
	})
	require.NoError(t, err)

	// Intentar INSERT directo con mismo content_hash → DB rechaza
	var hash []byte
	require.NoError(t, f.svc.Pool.QueryRow(ctx,
		`SELECT content_hash FROM observations WHERE id = $1`, o.ID).Scan(&hash))

	_, err = f.svc.Pool.Exec(ctx,
		`INSERT INTO observations (organization_id, project_id, content, observation_type, content_hash)
		 VALUES ($1, $2, 'bypass-app', 'note', $3)`,
		f.orgID, f.projectID, hash)
	require.Error(t, err, "DB UNIQUE PARTIAL debe rechazar duplicado aunque la app lo permita")
}

// Sabotaje: cross-org search no leaks (filtro organization_id en la query).
func TestSabotage_SearchHybrid_OrgIsolation(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.svc.Save(ctx, obssvc.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Content: "secreto de org A: contraseña foobar"})

	// Crear segunda org y project
	rec := &audit.PGRecorder{Pool: f.svc.Pool}
	orgS := &orgsvc.Service{Pool: f.svc.Pool, Audit: rec}
	projS := &projsvc.Service{Pool: f.svc.Pool, Audit: rec}
	org2, owner2, err := orgS.Create(ctx, "Other", "other", "x@x.com", "X")
	require.NoError(t, err)
	_, err = projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org2.ID, Name: "P", Slug: "p", ActorID: owner2.UserID,
	})
	require.NoError(t, err)

	// Search desde org2 con la misma query NO debe ver el secreto de org A
	results, err := f.svc.SearchHybrid(ctx, org2.ID, "secreto contraseña", 10)
	require.NoError(t, err)
	for _, r := range results {
		require.NotContains(t, r.Content, "foobar",
			"cross-org leak: org B no debe ver contenido de org A")
	}
}
