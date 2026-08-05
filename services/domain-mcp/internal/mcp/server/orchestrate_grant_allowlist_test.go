package mcpserver

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	flowsvc "nunezlagos/domain/internal/service/flow"
	orchsvc "nunezlagos/domain/internal/service/orchestrator"
)

// DOMAINSERV-218: el guard de allowlists tiene su propio test unitario en
// internal/service/flow, pero eso NO prueba que el handler lo INVOQUE. Sin este
// test, borrar la llamada a ValidarAllowlist en handleFlowGrantToken deja la suite
// entera en verde y el gate vuelve a emitir tokens que parecen scopeados y no lo
// están. Un guard cuyo cableado no se testea es un guard que no se ejecuta.

func grantToken(t *testing.T, h *orchestrateHandlers, flowRunID string, allowed []string) *mcp.CallToolResult {
	t.Helper()
	args := map[string]any{"flow_run_id": flowRunID, "session_id": testSessionID}
	if allowed != nil {
		raw := make([]any, 0, len(allowed))
		for _, p := range allowed {
			raw = append(raw, p)
		}
		args["allowed_paths"] = raw
	}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	res, err := h.handleFlowGrantToken(context.Background(), req)
	require.NoError(t, err)
	return res
}

func handlerConFlowActivo(t *testing.T) (*orchestrateHandlers, string) {
	t.Helper()
	svc := flowsvc.NewFlowTokenService([]byte("test-secret-0123456789"))
	fid := uuid.New()
	h := newValidateHandler(&fakeOrch{
		statusResp: &orchsvc.FlowStatusResponse{FlowRunID: fid, Status: "running"},
	}, svc)
	return h, fid.String()
}

func TestHandleFlowGrantToken_AllowlistSinPrefijoLiteral_EsRechazada(t *testing.T) {
	h, fid := handlerConFlowActivo(t)

	res := grantToken(t, h, fid, []string{"**/*.go"})

	require.True(t, res.IsError,
		"un glob con scope vacío no acota nada: emitir el token sería peor que negarlo")
	tc, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, "wildcard",
		"el mensaje tiene que decir POR QUÉ se rechazó, no solo que se rechazó")
}

func TestHandleFlowGrantToken_AllowlistConPrefijoDeDirectorio_EmiteElToken(t *testing.T) {
	h, fid := handlerConFlowActivo(t)

	res := grantToken(t, h, fid, []string{"services/domain-mcp/internal/observability/**"})

	require.False(t, res.IsError, "una allowlist bien formada tiene que seguir funcionando")
	tc, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, "token")
}

// Retrocompatibilidad: el flow normal no manda allowed_paths y no puede empezar a
// fallar por este guard. Es la mitad del cambio que se rompe si el guard se
// escribe de más.
func TestHandleFlowGrantToken_SinAllowlist_EmiteElToken(t *testing.T) {
	h, fid := handlerConFlowActivo(t)

	res := grantToken(t, h, fid, nil)

	require.False(t, res.IsError, "sin allowlist = flow normal sin batch-mode, tiene que pasar")
}
