//go:build integration

package knowledge_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/knowledge"
)

// DOMAINSERV-144: has_attachments existía como columna, se leía en los tres SELECT y
// se mapeaba a Doc.HasAttachments, pero el INSERT nunca la seteaba — o sea que exponía
// un dato que siempre decía false. Un doc creado por domain_attachment_index viene por
// definición de un adjunto, así que ahí tiene que ser true.
//
// DOMAINSERV-185: estos dos tests llamaban a Save con un context.Background() pelado, sin
// tx y sin app.current_project_id. Con el RLS de la 000287 ese camino devuelve
// "new row violates row-level security policy (SQLSTATE 42501)", y hacerlos pasar bajando la
// policy sería deformar la prueba. Pasan por conScopeDeProyecto, que es el mismo camino que
// usa producción: rlsProyecto en las 4 tools domain_knowledge_*, projectIDConScope en
// attachment_index, setProjectScope en project_index_submit y en orchestrator/analysis. Lo
// que se afirma no cambió; cambió el camino, y ahora es el real.
func TestKnowledge_Save_ConHasAttachments_LoPersiste(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	err := conScopeDeProyecto(t, f.svc.Pool, f.projectID, func(scoped context.Context) error {
		doc, _, err := f.svc.Save(scoped, knowledge.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.projectID, CreatedBy: &f.userID,
			Title: "contrato.md", Body: "texto extraído del adjunto",
			Source: "attachment", HasAttachments: true,
		})
		require.NoError(t, err)
		require.True(t, doc.HasAttachments, "el doc que viene de un adjunto tiene que declararlo")

		leido, _, err := f.svc.Get(scoped, f.projectID, doc.ID)
		require.NoError(t, err)
		require.True(t, leido.HasAttachments, "tiene que sobrevivir al round-trip, no solo al RETURNING")
		return nil
	})
	require.NoError(t, err)
}

// El default no cambia: un doc normal no declara adjuntos.
func TestKnowledge_Save_SinAdjunto_HasAttachmentsEnFalse(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	err := conScopeDeProyecto(t, f.svc.Pool, f.projectID, func(scoped context.Context) error {
		doc, _, err := f.svc.Save(scoped, knowledge.SaveInput{
			OrganizationID: f.orgID, ProjectID: f.projectID, CreatedBy: &f.userID,
			Title: "nota suelta", Body: "sin adjunto de por medio",
		})
		require.NoError(t, err)
		require.False(t, doc.HasAttachments)

		leido, _, err := f.svc.Get(scoped, f.projectID, doc.ID)
		require.NoError(t, err)
		require.False(t, leido.HasAttachments)
		return nil
	})
	require.NoError(t, err)
}
