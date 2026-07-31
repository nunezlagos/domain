//go:build integration

// Este test vive en el paquete interno y no en issuebuilder_test porque el bug
// no está en siguienteOrdinal —que es pura y ya tiene cobertura— sino en QUÉ
// conjunto de slugs se le pasa, o sea en la query y su scope.
package issuebuilder

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
)

type ordinalFixture struct {
	svc       *Service
	projectID uuid.UUID
	otroProy  uuid.UUID
	cleanup   func()
	insertReq func(t *testing.T, projectID uuid.UUID, slug string) uuid.UUID
	insertIsu func(t *testing.T, projectID, reqID uuid.UUID, slug string)
}

func setupOrdinal(t *testing.T) *ordinalFixture {
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

	nuevoProyecto := func(slug string) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO projects (name, slug) VALUES ($1, $1) RETURNING id`, slug,
		).Scan(&id))
		return id
	}

	f := &ordinalFixture{
		svc:       &Service{Pool: pool},
		projectID: nuevoProyecto("proy-ordinal"),
		otroProy:  nuevoProyecto("proy-vecino"),
		cleanup: func() {
			pool.Close()
			_ = pgC.Terminate(ctx)
		},
	}
	f.insertReq = func(t *testing.T, projectID uuid.UUID, slug string) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO sdd_requirements (project_id, slug, title) VALUES ($1, $2, $2) RETURNING id`,
			projectID, slug,
		).Scan(&id))
		return id
	}
	f.insertIsu = func(t *testing.T, projectID, reqID uuid.UUID, slug string) {
		t.Helper()
		_, err := pool.Exec(ctx,
			`INSERT INTO issues (project_id, req_id, slug, title) VALUES ($1, $2, $3, $3)`,
			projectID, reqID, slug,
		)
		require.NoError(t, err)
	}
	return f
}

// DOMAINSERV-211: el NAMESPACE del slug es el número de REQ —reReqNumber es
// ^REQ-(\d+), así que REQ-54 y REQ-54-tool-channels colapsan a "54"— pero el
// SCOPE de la query era req_id. Dos REQ con el mismo número compartían namespace
// sin compartir scope, y los dos emitían ordinal 1.
func TestService_NextIssueOrdinal_DosREQConElMismoNumero_NoComparteOrdinal(t *testing.T) {
	f := setupOrdinal(t)
	defer f.cleanup()
	ctx := context.Background()

	reqBase := f.insertReq(t, f.projectID, "REQ-54")
	f.insertIsu(t, f.projectID, reqBase, "issue-54.1-fix-derrame-de-cuerpo-en-tools-list")
	f.insertReq(t, f.projectID, "REQ-54-tool-channels")

	siguiente, err := f.svc.nextIssueOrdinal(ctx, "54", &f.projectID)
	require.NoError(t, err)

	require.Equal(t, 2, siguiente,
		"issue-54.1 ya está emitido bajo otro REQ del mismo número: reusarlo colisiona")
}

// El ordinal queda RESERVADO tras archivar: la query no filtra por status y eso
// es intencional, no una omisión.
func TestService_NextIssueOrdinal_IssueArchivado_SigueContando(t *testing.T) {
	f := setupOrdinal(t)
	defer f.cleanup()
	ctx := context.Background()

	reqID := f.insertReq(t, f.projectID, "REQ-77")
	f.insertIsu(t, f.projectID, reqID, "issue-77.1-ya-archivado")
	_, err := f.svc.Pool.Exec(ctx,
		`UPDATE issues SET status = 'archived' WHERE slug = 'issue-77.1-ya-archivado'`)
	require.NoError(t, err)

	siguiente, err := f.svc.nextIssueOrdinal(ctx, "77", &f.projectID)
	require.NoError(t, err)

	require.Equal(t, 2, siguiente, "un ordinal archivado no se recicla")
}

// El namespace por número no puede cruzar proyectos: sin el filtro explícito,
// issues de otro proyecto inflarían el máximo (la tabla issues no tiene RLS).
// Los dos REQ llevan slug distinto porque sdd_requirements_slug_idx es UNIQUE
// GLOBAL, no por proyecto — el mismo slug en dos proyectos es imposible, pero
// dos slugs con el mismo NÚMERO sí comparten namespace de ordinales.
func TestService_NextIssueOrdinal_IssueDeOtroProyecto_NoInflaElMaximo(t *testing.T) {
	f := setupOrdinal(t)
	defer f.cleanup()
	ctx := context.Background()

	reqVecino := f.insertReq(t, f.otroProy, "REQ-99-del-vecino")
	f.insertIsu(t, f.otroProy, reqVecino, "issue-99.7-del-vecino")
	f.insertReq(t, f.projectID, "REQ-99")

	siguiente, err := f.svc.nextIssueOrdinal(ctx, "99", &f.projectID)
	require.NoError(t, err)

	require.Equal(t, 1, siguiente,
		"el issue-99.7 es de otro proyecto: no debe contar para este namespace")
}

// Este es el guard del FILTRO POR NÚMERO, y hace falta explícitamente: los otros
// casos pasan igual si la query devuelve todos los issues del proyecto, porque
// cada uno usa un solo número. Sin este test, quitar el LIKE no rompe nada.
func TestService_NextIssueOrdinal_OtroNumeroDeREQ_NoInflaElMaximo(t *testing.T) {
	f := setupOrdinal(t)
	defer f.cleanup()
	ctx := context.Background()

	reqAlto := f.insertReq(t, f.projectID, "REQ-200")
	f.insertIsu(t, f.projectID, reqAlto, "issue-200.9-ordinal-alto")
	reqBajo := f.insertReq(t, f.projectID, "REQ-201")
	f.insertIsu(t, f.projectID, reqBajo, "issue-201.1-primero")

	siguiente, err := f.svc.nextIssueOrdinal(ctx, "201", &f.projectID)
	require.NoError(t, err)

	require.Equal(t, 2, siguiente,
		"issue-200.9 es de otro número de REQ: no comparte namespace con el 201")
}

func TestService_NextIssueOrdinal_SinIssuesPrevios_EmpiezaEnUno(t *testing.T) {
	f := setupOrdinal(t)
	defer f.cleanup()
	ctx := context.Background()

	f.insertReq(t, f.projectID, "REQ-123")

	siguiente, err := f.svc.nextIssueOrdinal(ctx, "123", &f.projectID)
	require.NoError(t, err)

	require.Equal(t, 1, siguiente)
}
