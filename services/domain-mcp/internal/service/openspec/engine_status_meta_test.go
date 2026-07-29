package openspec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-167: los dos casos devolvían el mismo "issue_id inválido en meta", así que
// el veredicto no distinguía un change que nunca pasó por el export —le falta el campo—
// de uno cuyo UUID está corrupto. Son causas distintas con arreglos distintos, y el
// change queda fuera de la auditoría de drift en los dos, en silencio.
//
// Los dos casos retornan antes de tocar la BD, así que un Engine vacío alcanza.
func TestStatus_MetaSinIssueID_DaUnMotivoDistintoQueUnUUIDRoto(t *testing.T) {
	e := &Engine{}

	sinCampo := e.Status(context.Background(), []File{{
		Path:    "openspec/changes/hecho-a-mano/.openspec.yaml",
		Content: "change: hecho-a-mano\nstatus: active\nepic: DOMAINSERV-62\n",
	}})
	require.Len(t, sinCampo, 1)
	require.Equal(t, "error", sinCampo[0].Verdict)
	require.Contains(t, sinCampo[0].Reason, "sin domain.issue_id")
	require.Contains(t, sinCampo[0].Reason, "export",
		"el motivo tiene que decir cómo se arregla, no solo que está mal")

	uuidRoto := e.Status(context.Background(), []File{{
		Path:    "openspec/changes/uuid-roto/.openspec.yaml",
		Content: "schema: spec-driven\ndomain:\n  issue_id: no-es-un-uuid\n",
	}})
	require.Len(t, uuidRoto, 1)
	require.Equal(t, "error", uuidRoto[0].Verdict)
	require.Contains(t, uuidRoto[0].Reason, "no es un UUID")
	require.NotContains(t, uuidRoto[0].Reason, "sin domain.issue_id",
		"un UUID corrupto no se diagnostica igual que un campo ausente")
}

// El .openspec.yaml faltante ya tenía su propio motivo y no se toca: este test lo fija
// para que la separación de los otros dos no lo colapse con ellos.
func TestStatus_SinArchivoDeMeta_ConservaSuMotivoPropio(t *testing.T) {
	e := &Engine{}
	res := e.Status(context.Background(), []File{{
		Path:    "openspec/changes/sin-meta/proposal.md",
		Content: "# algo\n",
	}})
	require.Len(t, res, 1)
	require.Equal(t, "error", res[0].Verdict)
	require.Contains(t, res[0].Reason, "falta .openspec.yaml")
}
