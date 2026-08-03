//go:build integration

// DOMAINSERV-227, mitad accionable: `domain_knowledge_save` era una escritura SIN INVERSA.
// El SQL, el service y el filtro de la búsqueda ya existían; faltaba la tool MCP, así que
// borrar un documento exigía SQL contra producción. Un agente de ingesta que falló dejó un
// documento con el título real de un doc y un cuerpo de 15 caracteres, indexado y buscable.
//
// El defecto que NO se podía propagar al exponerlo: `SoftDelete` resolvía el documento SOLO
// por UUID. Es el mismo agujero que DOMAINSERV-217 acababa de cerrar en las tools de verify,
// donde un principal podía mutar checkpoints de otro proyecto. Estos tests fijan el scope
// contra una base real: que el filtro esté escrito lo verifica el guard de fuente; que
// FUNCIONE, esto.
package knowledge_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/knowledge"
)

func TestKnowledge_SoftDelete_ConIDDeOtroProyecto_NoBorra(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	ctx := context.Background()

	doc, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.proyectoA,
		Title: "secreto de A", Body: "contenido que B no debe poder borrar",
	})
	require.NoError(t, err)

	// B pide borrar un documento de A pasando su propio project_id
	err = f.svc.SoftDelete(ctx, f.proyectoB, doc.ID, f.userID)
	require.ErrorIs(t, err, knowledge.ErrNotFound,
		"un documento de otro proyecto NO se borra: resolver solo por UUID es el IDOR de DOMAINSERV-217")

	// y sigue vivo para su dueño, que es lo que hace del caso un borrado NO ocurrido
	// en vez de un error después del daño
	_, _, err = f.svc.Get(ctx, f.proyectoA, doc.ID)
	require.NoError(t, err, "el documento de A tiene que seguir existiendo tras el intento de B")
}

func TestKnowledge_SoftDelete_DelProyectoPropio_BorraYSaleDeLaBusqueda(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	ctx := context.Background()

	doc, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.proyectoA,
		Title: "documento basura de una ingesta fallida", Body: "TEST_PROBE_ONLY zarigüeya",
	})
	require.NoError(t, err)

	// aparece en la búsqueda ANTES del borrado: sin esto el test de abajo pasaría
	// también con una búsqueda que nunca devuelve nada
	antes, err := f.svc.SearchHybrid(ctx, f.proyectoA, "zarigüeya", 10)
	require.NoError(t, err)
	require.NotEmpty(t, antes, "el documento tiene que ser encontrable antes de borrarlo")

	require.NoError(t, f.svc.SoftDelete(ctx, f.proyectoA, doc.ID, f.userID))

	_, _, err = f.svc.Get(ctx, f.proyectoA, doc.ID)
	require.ErrorIs(t, err, knowledge.ErrNotFound)

	// la propiedad que le importa a quien pidió el borrado: la basura deja de
	// contaminar las búsquedas. knowledge_chunks NO tiene deleted_at, así que esto
	// depende de que la búsqueda filtre por el deleted_at del doc PADRE
	despues, err := f.svc.SearchHybrid(ctx, f.proyectoA, "zarigüeya", 10)
	require.NoError(t, err)
	for _, r := range despues {
		require.NotEqual(t, doc.ID, r.DocumentID,
			"los chunks de un doc borrado siguen apareciendo en la búsqueda: el soft-delete no alcanzó")
	}
}

func TestKnowledge_SoftDelete_IDInexistente_DevuelveNotFound(t *testing.T) {
	f, cleanup := setupDosProyectos(t)
	defer cleanup()
	ctx := context.Background()

	doc, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.proyectoA, Title: "T", Body: "y",
	})
	require.NoError(t, err)

	require.NoError(t, f.svc.SoftDelete(ctx, f.proyectoA, doc.ID, f.userID))
	// el segundo borrado ya no encuentra fila viva: idempotente hacia el caller,
	// pero reportado y no silencioso
	require.ErrorIs(t, f.svc.SoftDelete(ctx, f.proyectoA, doc.ID, f.userID), knowledge.ErrNotFound)
}
