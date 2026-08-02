//go:build integration

package knowledge_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/store/txctx"
)

// conScopeDeProyecto abre una tx con app.current_project_id seteado y la inyecta en el ctx,
// igual que withProjectTxHandler en el wireup MCP. Es el único camino por el que deben
// pasar lecturas y escrituras de knowledge cuando el RLS de la parte B esté activo.
func conScopeDeProyecto(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID, fn func(context.Context) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT set_config('app.current_project_id', $1, true)`, projectID.String())
	require.NoError(t, err)
	if err := fn(txctx.WithTxContext(ctx, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DOMAINSERV-185: el test que prueba que la defensa EXISTE, no que el WHERE está escrito.
// Sin el SET LOCAL la búsqueda tiene que quedar en cero: si devuelve filas, la única capa
// que filtra es el WHERE de la query y un refactor que lo borre no se nota.
//
// QUEDA ROJO A PROPÓSITO en la parte A: hoy knowledge_docs/knowledge_chunks no tienen RLS
// (verificado sobre la base migrada: relrowsecurity=false en ambas), así que sin GUC la
// búsqueda devuelve el doc igual. Es el test que la migración de la parte B pone en verde.
func TestKnowledge_SearchHybrid_SinGUCDeProyecto_NoDevuelveFilas(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	ctx := context.Background()

	err := conScopeDeProyecto(t, f.svc.Pool, f.proyectoA, func(scoped context.Context) error {
		_, _, err := f.svc.Save(scoped, knowledge.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.proyectoA, CreatedBy: &f.userID,
			Title: "Acuerdo de A", Body: "clausula reservada del acuerdo marco",
		})
		return err
	})
	require.NoError(t, err,
		"el escritor CON el GUC seteado tiene que poder insertar: si esto falla, el WITH CHECK "+
			"está rechazando a los escritores legítimos y domain_knowledge_save queda roto")

	err = conScopeDeProyecto(t, f.svc.Pool, f.proyectoA, func(scoped context.Context) error {
		res, err := f.svc.SearchHybrid(scoped, f.proyectoA, "clausula reservada", 20)
		require.NoError(t, err)
		require.NotEmpty(t, res, "con el GUC del proyecto dueño la búsqueda TIENE que devolver su doc")
		return nil
	})
	require.NoError(t, err)

	sinGUC, err := f.svc.SearchHybrid(ctx, f.proyectoA, "clausula reservada", 20)
	require.NoError(t, err)
	require.Empty(t, sinGUC,
		"SABOTAJE: sin app.current_project_id la búsqueda devolvió filas, así que no hay "+
			"segunda capa: el aislamiento depende solo del WHERE de la query")
}

// Save abría su propia tx contra el pool, ignorando la del contexto. Bajo el RLS de la
// parte B ese INSERT corre en otra conexión, sin el GUC que seteó el wrapper, y el WITH
// CHECK lo rechaza: domain_knowledge_save quedaría roto. El rollback de la tx externa es
// lo que delata el commit propio.
func TestKnowledge_Save_ConTxEnElContexto_NoCommiteaPorSuCuenta(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	ctx := context.Background()

	tx, err := f.svc.Pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	doc, _, err := f.svc.Save(txctx.WithTxContext(ctx, tx), knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.proyectoA, CreatedBy: &f.userID,
		Title: "Borrador que no debe sobrevivir", Body: "contenido de una tx abortada",
	})
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))

	_, _, err = f.svc.Get(ctx, f.proyectoA, doc.ID)
	require.ErrorIs(t, err, knowledge.ErrNotFound,
		"Save ignoró la tx del contexto y commiteó por su cuenta: el doc sobrevivió al "+
			"rollback, así que el INSERT corre fuera del alcance del GUC de proyecto")
}
