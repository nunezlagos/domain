//go:build integration

package knowledge_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/knowledge"
	projsvc "nunezlagos/domain/internal/service/project"
	"nunezlagos/domain/internal/store/txctx"
)

type fix struct {
	svc       *knowledge.Service
	orgID     uuid.UUID
	projectID uuid.UUID
	userID    uuid.UUID
}

// enScope abre una tx con app.current_project_id seteado al proyecto del fixture y devuelve
// el ctx que la lleva, más su cierre.
//
// DOMAINSERV-185: con el RLS de la 000287 un Save contra un context.Background() pelado lo
// rechaza el WITH CHECK ("new row violates row-level security policy"), así que este es el
// único camino por el que un test puede escribir knowledge — y es el mismo que usa
// producción vía rlsProyecto. El cierre es un Rollback y no un Commit a propósito: cada test
// levanta su propio container, así que no hay nada que valga la pena persistir, y un rollback
// no puede dejar la tx colgada reteniendo el lock que después el cleanup espera.
func (f *fix) enScope(t *testing.T) (context.Context, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.svc.Pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT set_config('app.current_project_id', $1, true)`, f.projectID.String())
	require.NoError(t, err)
	return txctx.WithTxContext(ctx, tx), func() { _ = tx.Rollback(ctx) }
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
	org, owner, _ := seedOrgUser(ctx, pools.App, "Acme", "acme", "o@x.com", "O")
	proj, _ := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})

	// sin Dim el embedder no produce vector y el insert queda sin embedding: la
	// dimensión la declara el caller desde la migración que fijó las columnas
	svc := &knowledge.Service{
		Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{Dim: dmigrate.EmbeddingDim},
	}
	return &fix{svc: svc, orgID: org.ID, projectID: proj.ID, userID: owner.UserID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestKnowledge_Save_ShortDocSingleChunk(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx, done := f.enScope(t)
	defer done()
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
	ctx, done := f.enScope(t)
	defer done()
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
	ctx, done := f.enScope(t)
	defer done()
	// el error de Save deja de ignorarse: con `doc, _, _ :=` un INSERT rechazado por RLS
	// aparecía como un SIGSEGV al desreferenciar doc nil, no como el error que fue
	doc, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "T", Body: strings.Repeat("hola amigo. ", 300),
	})
	require.NoError(t, err)
	got, chunks, err := f.svc.Get(ctx, f.projectID, doc.ID)
	require.NoError(t, err)
	require.Equal(t, doc.ID, got.ID)
	require.True(t, len(chunks) >= 1)
}

func TestKnowledge_SearchHybrid_Semantic(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx, done := f.enScope(t)
	defer done()
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "A", Body: "El sistema usa pgvector para búsqueda semántica con cosine.",
	})
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "B", Body: "El clima ayer fue lluvioso en Santiago.",
	})

	results, err := f.svc.SearchHybrid(ctx, f.projectID, "pgvector búsqueda", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Contains(t, strings.ToLower(results[0].Snippet), "pgvector")
}

func TestKnowledge_SearchHybrid_NopEmbedderDegradesToTSVector(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx, done := f.enScope(t)
	defer done()
	f.svc.Embedder = llm.NopEmbedder{Dim: dmigrate.EmbeddingDim}
	_, _, _ = f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID,
		Title: "T", Body: "documentación del módulo de memoria de domain.",
	})
	results, err := f.svc.SearchHybrid(ctx, f.projectID, "memoria domain", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
}

func TestKnowledge_ListByProject(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx, done := f.enScope(t)
	defer done()
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

// DOMAINSERV-185: dos cambios acá y los dos importan. (1) El test pasa por
// conScopeDeProyecto: con el RLS de la 000287 un Save sin app.current_project_id lo rechaza
// el WITH CHECK, y ese es el camino que producción ya usa. (2) El error de Save DEJA de
// ignorarse — decía `doc, _, _ :=`, así que cuando el INSERT empezó a fallar el test no
// reportaba el error de RLS sino un SIGSEGV al desreferenciar doc nil. Tragarse un error
// convierte un fallo diagnosticable en un panic que no dice nada.
func TestKnowledge_SoftDelete(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	err := conScopeDeProyecto(t, f.svc.Pool, f.projectID, func(scoped context.Context) error {
		doc, _, err := f.svc.Save(scoped, knowledge.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.projectID, Title: "T", Body: "y",
		})
		require.NoError(t, err)

		require.NoError(t, f.svc.SoftDelete(scoped, f.projectID, doc.ID, f.userID))
		_, _, err = f.svc.Get(scoped, f.projectID, doc.ID)
		require.ErrorIs(t, err, knowledge.ErrNotFound)
		return nil
	})
	require.NoError(t, err)
}

// Sabotaje: cross-org search no leak
// Sabotaje: si Embed falla, Save NO crea doc parcial (atómico)
type failingEmbedder struct{ dim int }

func (failingEmbedder) Dimensions() int { return 1536 }
func (failingEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, errors.New("synthetic embedder failure")
}
func (failingEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
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
