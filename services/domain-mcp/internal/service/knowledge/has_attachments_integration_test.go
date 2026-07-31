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
func TestKnowledge_Save_ConHasAttachments_LoPersiste(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	doc, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, CreatedBy: &f.userID,
		Title: "contrato.md", Body: "texto extraído del adjunto",
		Source: "attachment", HasAttachments: true,
	})
	require.NoError(t, err)
	require.True(t, doc.HasAttachments, "el doc que viene de un adjunto tiene que declararlo")

	leido, _, err := f.svc.Get(ctx, f.projectID, doc.ID)
	require.NoError(t, err)
	require.True(t, leido.HasAttachments, "tiene que sobrevivir al round-trip, no solo al RETURNING")
}

// El default no cambia: un doc normal no declara adjuntos.
func TestKnowledge_Save_SinAdjunto_HasAttachmentsEnFalse(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	doc, _, err := f.svc.Save(ctx, knowledge.SaveInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, CreatedBy: &f.userID,
		Title: "nota suelta", Body: "sin adjunto de por medio",
	})
	require.NoError(t, err)
	require.False(t, doc.HasAttachments)

	leido, _, err := f.svc.Get(ctx, f.projectID, doc.ID)
	require.NoError(t, err)
	require.False(t, leido.HasAttachments)
}
