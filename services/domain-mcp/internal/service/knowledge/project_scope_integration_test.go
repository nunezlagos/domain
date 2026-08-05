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

	// sin Dim el embedder no produce vector y el insert queda sin embedding: la
	// dimensión la declara el caller desde la migración que fijó las columnas
	svc := &knowledge.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{Dim: dmigrate.EmbeddingDim}}

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
// DOMAINSERV-185, y el detalle es el que decide si este test sigue sirviendo: la búsqueda
// "desde B" corre con el GUC de **A** seteado, no con el de B. Con el GUC de B el RLS de la
// 000287 ya esconde los docs de A, así que la aserción pasaría incluso con el WHERE de
// DOMAINSERV-182 borrado — la segunda capa enmascararía al test de la primera, que es el
// modo de falla que este repo ya se comió una vez. Con el GUC de A el RLS está neutralizado
// a propósito y lo único que puede filtrar es el WHERE. Cada capa conserva su propio rojo.
func TestKnowledge_SearchHybrid_DocDeOtroProyecto_NoAparece(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()

	const secreto = "clausula confidencial del contrato de un cliente"
	require.NoError(t, conScopeDeProyecto(t, f.svc.Pool, f.proyectoA, func(scoped context.Context) error {
		_, _, err := f.svc.Save(scoped, knowledge.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.proyectoA, CreatedBy: &f.userID,
			Title: "Contrato privado de A", Body: secreto,
		})
		require.NoError(t, err)

		desdeA, err := f.svc.SearchHybrid(scoped, f.proyectoA, "clausula confidencial", 20)
		require.NoError(t, err)
		require.NotEmpty(t, desdeA, "el proyecto dueño SÍ tiene que encontrar su propio doc")

		desdeB, err := f.svc.SearchHybrid(scoped, f.proyectoB, "clausula confidencial", 20)
		require.NoError(t, err)
		require.Empty(t, desdeB,
			"FUGA CROSS-TENANT: el WHERE por proyecto no filtró — con el GUC de A el RLS no "+
				"está tapando nada, así que esto mide el filtro de la query y nada más")
		return nil
	}))
}

// Mismo aislamiento por la rama BM25, que es la que corre cuando el embedder no
// devuelve un vector útil. Es un camino distinto en el SQL (SearchBm25, no
// SearchHybrid) y por eso necesita su propia prueba: arreglar uno y no el otro deja el
// leak abierto justo cuando el embedder está degradado.
func TestKnowledge_SearchBm25_SinVector_TampocoCruzaDeProyecto(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	svcSinVector := &knowledge.Service{
		Pool: f.svc.Pool, Audit: f.svc.Audit, Embedder: llm.NopEmbedder{Dim: dmigrate.EmbeddingDim},
	}

	// mismo criterio que el test de arriba: el GUC es de A, así el RLS no puede tapar el
	// leak y lo que se mide es el WHERE de la rama BM25
	require.NoError(t, conScopeDeProyecto(t, f.svc.Pool, f.proyectoA, func(scoped context.Context) error {
		_, _, err := svcSinVector.Save(scoped, knowledge.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.proyectoA, CreatedBy: &f.userID,
			Title: "Nota privada de A", Body: "presupuesto interno reservado",
		})
		require.NoError(t, err)

		desdeB, err := svcSinVector.SearchHybrid(scoped, f.proyectoB, "presupuesto interno", 20)
		require.NoError(t, err)
		require.Empty(t, desdeB,
			"FUGA CROSS-TENANT por la rama BM25: el fallback sin vector no filtra por proyecto")
		return nil
	}))
}

// Get por id era un IDOR: con el id en mano se leía el doc de otro proyecto. Ahora el
// id sin el proyecto dueño no resuelve.
func TestKnowledge_Get_ConIDDeOtroProyecto_DevuelveNotFound(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	// el GUC de A también acá: con el de B el RLS haría desaparecer el doc y el ErrNotFound
	// llegaría por permiso en vez de por el WHERE, que es lo que este test vino a fijar
	require.NoError(t, conScopeDeProyecto(t, f.svc.Pool, f.proyectoA, func(scoped context.Context) error {
		doc, _, err := f.svc.Save(scoped, knowledge.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.proyectoA, CreatedBy: &f.userID,
			Title: "Doc de A", Body: "contenido de A",
		})
		require.NoError(t, err)

		_, _, err = f.svc.Get(scoped, f.proyectoA, doc.ID)
		require.NoError(t, err, "el proyecto dueño SÍ tiene que poder leer su doc")

		_, _, err = f.svc.Get(scoped, f.proyectoB, doc.ID)
		require.ErrorIs(t, err, knowledge.ErrNotFound,
			"IDOR: con el id de un doc ajeno, otro proyecto obtuvo el documento")
		return nil
	}))
}
