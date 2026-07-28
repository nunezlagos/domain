//go:build integration

package knowledge_test

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
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/knowledge"
	projsvc "nunezlagos/domain/internal/service/project"
)

// DOMAINSERV-182: este es el test que prueba la defensa. El guard de fuente
// (project_scope_guard_test.go) verifica que el filtro esté escrito; este verifica que
// FUNCIONE contra una base real. Sin él, "filtra por proyecto" es una afirmación sobre
// el texto de una query, no sobre su comportamiento.

type fixDosProyectos struct {
	svc       *knowledge.Service
	orgID     uuid.UUID
	userID    uuid.UUID
	proyectoA uuid.UUID
	proyectoB uuid.UUID
}

func setupDosProyectos(t *testing.T) (*fixDosProyectos, func()) {
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
	org, owner := mustSeedOrgUser(t, ctx, pools.App, "Acme", "acme", "scope@x.com", "O")

	projA, err := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Proyecto A", Slug: "proyecto-a", ActorID: owner.UserID,
	})
	require.NoError(t, err)
	projB, err := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Proyecto B", Slug: "proyecto-b", ActorID: owner.UserID,
	})
	require.NoError(t, err)

	svc := &knowledge.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}

	return &fixDosProyectos{
			svc: svc, orgID: org.ID, userID: owner.UserID,
			proyectoA: projA.ID, proyectoB: projB.ID,
		}, func() {
			pools.Close()
			_ = pgC.Terminate(ctx)
		}
}

// El caso central del ticket: un doc del proyecto A no puede aparecer buscando en B.
// Antes de este fix aparecía, porque las queries JOINeaban knowledge_docs solo para
// deleted_at IS NULL.
func TestKnowledge_SearchHybrid_DocDeOtroProyecto_NoAparece(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	ctx := context.Background()

	const secreto = "clausula confidencial del contrato de un cliente"
	_, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.proyectoA, CreatedBy: &f.userID,
		Title: "Contrato privado de A", Body: secreto,
	})
	require.NoError(t, err)

	desdeA, err := f.svc.SearchHybrid(ctx, f.proyectoA, "clausula confidencial", 20)
	require.NoError(t, err)
	require.NotEmpty(t, desdeA, "el proyecto dueño SÍ tiene que encontrar su propio doc")

	desdeB, err := f.svc.SearchHybrid(ctx, f.proyectoB, "clausula confidencial", 20)
	require.NoError(t, err)
	require.Empty(t, desdeB,
		"FUGA CROSS-TENANT: el proyecto B recibió contenido del proyecto A")
}

// Mismo aislamiento por la rama BM25, que es la que corre cuando el embedder no
// devuelve un vector útil. Es un camino distinto en el SQL (SearchBm25, no
// SearchHybrid) y por eso necesita su propia prueba: arreglar uno y no el otro deja el
// leak abierto justo cuando el embedder está degradado.
func TestKnowledge_SearchBm25_SinVector_TampocoCruzaDeProyecto(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	ctx := context.Background()

	svcSinVector := &knowledge.Service{
		Pool: f.svc.Pool, Audit: f.svc.Audit, Embedder: llm.NopEmbedder{},
	}

	_, _, err := svcSinVector.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.proyectoA, CreatedBy: &f.userID,
		Title: "Nota privada de A", Body: "presupuesto interno reservado",
	})
	require.NoError(t, err)

	desdeB, err := svcSinVector.SearchHybrid(ctx, f.proyectoB, "presupuesto interno", 20)
	require.NoError(t, err)
	require.Empty(t, desdeB,
		"FUGA CROSS-TENANT por la rama BM25: el fallback sin vector no filtra por proyecto")
}

// Get por id era un IDOR: con el id en mano se leía el doc de otro proyecto. Ahora el
// id sin el proyecto dueño no resuelve.
func TestKnowledge_Get_ConIDDeOtroProyecto_DevuelveNotFound(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	ctx := context.Background()

	doc, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.proyectoA, CreatedBy: &f.userID,
		Title: "Doc de A", Body: "contenido de A",
	})
	require.NoError(t, err)

	_, _, err = f.svc.Get(ctx, f.proyectoA, doc.ID)
	require.NoError(t, err, "el proyecto dueño SÍ tiene que poder leer su doc")

	_, _, err = f.svc.Get(ctx, f.proyectoB, doc.ID)
	require.ErrorIs(t, err, knowledge.ErrNotFound,
		"IDOR: con el id de un doc ajeno, otro proyecto obtuvo el documento")
}
