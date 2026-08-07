//go:build integration

// Tests de RLS de las tablas de compliance (issue-56.4).
//
// POR QUÉ SON DE INTEGRACIÓN Y NO UNITARIOS: la suite unitaria pasa ENTERA con el RLS mal puesto.
// Se midió el 2026-08-06 en DOMAINSERV-240 — la 000288 puso webhooks bajo RLS, el service nunca
// escribió project_id, y los tests de integración quedaron rojos en main sin que nadie lo notara
// porque 2774 tests unitarios seguían en verde. Un RLS solo se verifica contra Postgres.
package compliance_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
)

type fixture struct {
	pools     *db.Pools
	proyectoA uuid.UUID
	proyectoB uuid.UUID
	userID    uuid.UUID
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
	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	f := &fixture{pools: pools}
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO users (email, name, role) VALUES ('o@x.com','O','owner') RETURNING id`,
	).Scan(&f.userID))
	f.proyectoA = crearProyecto(t, ctx, pools.App, "proyecto-a")
	f.proyectoB = crearProyecto(t, ctx, pools.App, "proyecto-b")

	return f, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func crearProyecto(t *testing.T, ctx context.Context, pool *pgxpool.Pool, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ($1, $1) RETURNING id`, slug).Scan(&id))
	return id
}

// enScope abre una tx con app.current_project_id seteado, que es el único camino por el que se
// puede escribir en las tablas por proyecto — el mismo que usa producción vía rlsProyecto.
func (f *fixture) enScope(t *testing.T, projectID uuid.UUID) (context.Context, pgx.Tx, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pools.App.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT set_config('app.current_project_id', $1, true)`, projectID.String())
	require.NoError(t, err)
	return ctx, tx, func() { _ = tx.Rollback(ctx) }
}

// crearMarco inserta un marco en el catálogo. Va por el pool de la app a propósito: si el catálogo
// quedara bajo RLS, esta escritura fallaría y el test lo diría.
func (f *fixture) crearMarco(t *testing.T, slug, tipo string, obligatorio bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pools.App.QueryRow(context.Background(),
		`INSERT INTO compliance_frameworks (slug, nombre, tipo, obligatorio)
		 VALUES ($1, $1, $2, $3) RETURNING id`, slug, tipo, obligatorio).Scan(&id))
	return id
}

// REQ-4 — el catálogo se lee SIN GUC de proyecto.
//
// Si alguien le pone RLS al catálogo, las consultas devuelven cero filas SIN ERROR y el sistema
// parece no tener marcos cargados. Es el modo de falla de la 000287 y la 000288, y el síntoma es
// indistinguible de "no hay datos".
func TestCatalogo_SeLeeSinScopeDeProyecto(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	f.crearMarco(t, "ley-21719", "ley", true)
	f.crearMarco(t, "iso-27001", "norma_tecnica", false)

	var n int
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`SELECT count(*) FROM compliance_frameworks WHERE deleted_at IS NULL`).Scan(&n))

	assert.Equal(t, 2, n,
		"el catálogo tiene que ser legible sin app.current_project_id: si devuelve 0, alguien le "+
			"puso RLS y el sistema va a parecer no tener marcos cargados, sin ningún error")
}

// REQ-1 — la ausencia de fila significa NO APLICA. Es el opt-in real, lo contrario del modelo de
// skills, donde lo global auto-aplica y project_skills solo excluye.
func TestOptIn_ProyectoSinDeclarar_NoTieneMarcos(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	f.crearMarco(t, "ley-21719", "ley", true)
	f.crearMarco(t, "gdpr", "reglamento", true)

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()

	var n int
	require.NoError(t, tx.QueryRow(ctx,
		`SELECT count(*) FROM project_compliance_frameworks WHERE activo`).Scan(&n))

	assert.Zero(t, n,
		"un proyecto que no declaró nada no está afecto a ningún marco, aunque el catálogo tenga "+
			"leyes cargadas")
}

// REQ-4 — las declaraciones de otro proyecto no son visibles.
func TestRLS_DeclaracionesDeOtroProyecto_NoSonVisibles(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	marco := f.crearMarco(t, "ley-21719", "ley", true)

	ctxB, txB, cerrarB := f.enScope(t, f.proyectoB)
	_, err := txB.Exec(ctxB,
		`INSERT INTO project_compliance_frameworks (project_id, framework_id) VALUES ($1, $2)`,
		f.proyectoB, marco)
	require.NoError(t, err)
	require.NoError(t, txB.Commit(ctxB))
	cerrarB()

	ctxA, txA, cerrarA := f.enScope(t, f.proyectoA)
	defer cerrarA()

	var n int
	require.NoError(t, txA.QueryRow(ctxA,
		`SELECT count(*) FROM project_compliance_frameworks`).Scan(&n))

	assert.Zero(t, n, "el proyecto A está viendo las declaraciones del proyecto B")
}

// REQ-4, sabotaje — escribir el estado de un control SIN scope tiene que ser rechazado, no quedar
// como fila huérfana de proyecto.
func TestRLS_EstadoDeControlSinScope_EsRechazado(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	var control uuid.UUID
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`INSERT INTO compliance_controls (slug, nombre) VALUES ('cifrado-en-reposo','Cifrado')
		 RETURNING id`).Scan(&control))

	_, err := f.pools.App.Exec(ctx,
		`INSERT INTO project_control_status (project_id, control_id, estado)
		 VALUES ($1, $2, 'ok')`, f.proyectoA, control)

	require.Error(t, err,
		"sin app.current_project_id el WITH CHECK de la policy tiene que rechazar la escritura")
	assert.Contains(t, err.Error(), "row-level security")
}

// REQ-2 — el crosswalk: un control satisface varios marcos, y cada uno conserva SU referencia.
func TestCrosswalk_UnControlSatisfaceVariosMarcos_ConSuPropiaReferencia(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	gdpr := f.crearMarco(t, "gdpr", "reglamento", true)
	iso := f.crearMarco(t, "iso-27001", "norma_tecnica", false)
	var control uuid.UUID
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`INSERT INTO compliance_controls (slug, nombre) VALUES ('cifrado-en-reposo','Cifrado')
		 RETURNING id`).Scan(&control))

	for _, v := range []struct {
		marco      uuid.UUID
		referencia string
	}{{gdpr, "Art. 32"}, {iso, "A.8.24"}} {
		_, err := f.pools.App.Exec(ctx,
			`INSERT INTO framework_controls (framework_id, control_id, referencia)
			 VALUES ($1, $2, $3)`, v.marco, control, v.referencia)
		require.NoError(t, err)
	}

	rows, err := f.pools.App.Query(ctx,
		`SELECT cf.slug, fc.referencia FROM framework_controls fc
		 JOIN compliance_frameworks cf ON cf.id = fc.framework_id
		 WHERE fc.control_id = $1 ORDER BY cf.slug`, control)
	require.NoError(t, err)
	defer rows.Close()

	refs := map[string]string{}
	for rows.Next() {
		var slug, ref string
		require.NoError(t, rows.Scan(&slug, &ref))
		refs[slug] = ref
	}

	assert.Equal(t, "Art. 32", refs["gdpr"])
	assert.Equal(t, "A.8.24", refs["iso-27001"],
		"cada marco cita el mismo control con su propia referencia: es lo que permite evaluarlo una "+
			"vez y reportarlo en todos")
}

// REQ-6 — dos ediciones de la misma norma conviven; el UNIQUE lleva la edición.
func TestCatalogo_DosEdicionesDeLaMismaNorma_Conviven(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	for _, edicion := range []string{"2013", "2022"} {
		_, err := f.pools.App.Exec(ctx,
			`INSERT INTO compliance_frameworks (slug, nombre, tipo, edicion)
			 VALUES ('iso-27001','ISO 27001','norma_tecnica',$1)`, edicion)
		require.NoError(t, err, "la edición %s debería poder convivir", edicion)
	}

	_, err := f.pools.App.Exec(ctx,
		`INSERT INTO compliance_frameworks (slug, nombre, tipo, edicion)
		 VALUES ('iso-27001','ISO 27001','norma_tecnica','2022')`)
	require.Error(t, err, "la misma norma en la misma edición sí debe ser única")
}

// REQ-6 — una norma no es territorial: jurisdiccion NULL tiene que ser válido.
func TestCatalogo_NormaSinJurisdiccion_SeAcepta(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	_, err := f.pools.App.Exec(context.Background(),
		`INSERT INTO compliance_frameworks (slug, nombre, tipo, jurisdiccion, obligatorio, certificable)
		 VALUES ('iso-27001','ISO 27001','norma_tecnica', NULL, false, true)`)

	require.NoError(t, err,
		"ISO no es de ningún país: forzar una jurisdicción obligaría a inventar una")
}

// El CHECK de tipo y el de fuente_tipo son el guard contra valores inventados desde el código.
func TestCatalogo_TipoYFuenteInvalidos_SeRechazan(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.pools.App.Exec(ctx,
		`INSERT INTO compliance_frameworks (slug, nombre, tipo) VALUES ('x','X','recomendacion')`)
	require.Error(t, err, "un tipo fuera del CHECK tiene que rechazarse")

	_, err = f.pools.App.Exec(ctx,
		`INSERT INTO compliance_frameworks (slug, nombre, tipo, fuente_tipo)
		 VALUES ('y','Y','ley','copiar_todo')`)
	require.Error(t, err,
		"fuente_tipo es el guard de copyright: un valor libre lo volvería decorativo")
}
