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

// DOMAINSERV-256: el fail-open que ningún test veía. allowedPathsDelRequest hacía un type
// assert a []any y devolvía nil ante cualquier otra cosa — o sea "sin restricción de path".
// Un cliente que serializara la allowlist como string JSON recibía un token SIN scope
// creyendo estar confinado, y no había forma de notarlo desde la respuesta del tool: el
// server no la eco. Se detectó decodificando el claim `p` del marker.
//
// grantToken() no sirve acá: arma el array bien tipado a propósito. Estos casos tienen que
// meter la basura cruda en los argumentos.
func grantTokenCrudo(t *testing.T, h *orchestrateHandlers, flowRunID string, allowed any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"flow_run_id":   flowRunID,
		"session_id":    testSessionID,
		"allowed_paths": allowed,
	}
	res, err := h.handleFlowGrantToken(context.Background(), req)
	require.NoError(t, err)
	return res
}

func TestHandleFlowGrantToken_AllowedPathsComoString_EsErrorYNoEmite(t *testing.T) {
	h, fid := handlerConFlowActivo(t)

	res := grantTokenCrudo(t, h, fid, "services/domain-mcp/**")

	require.True(t, res.IsError, "un allowed_paths mal tipado NO puede degradar a 'sin scope' en silencio")
	tc, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, "array de strings")
	// el error explica el modo de falla, no solo que rechazó: sin esto el mensaje sería
	// "input inválido" y el cliente que serializa mal no sabría qué corregir
	require.Contains(t, tc.Text, "sin restricción de path")
}

func TestHandleFlowGrantToken_AllowedPathsConItemNoString_EsErrorYNoEmite(t *testing.T) {
	h, fid := handlerConFlowActivo(t)

	res := grantTokenCrudo(t, h, fid, []any{"services/domain-mcp/**", 42})

	require.True(t, res.IsError, "descartar el item inválido achicaría el territorio declarado sin avisar")
	tc, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, "allowed_paths[1]")
}

func TestHandleFlowGrantToken_AllowedPathsConStringVacio_EsError(t *testing.T) {
	h, fid := handlerConFlowActivo(t)

	res := grantTokenCrudo(t, h, fid, []any{"services/domain-mcp/**", "   "})

	require.True(t, res.IsError, "un glob en blanco es territorio declarado que se perdería")
}

// La otra mitad, y la que hace seguro el cambio: ausente sigue significando "sin
// restricción". Si esto se rompe, todo flow que no declara partición deja de poder editar.
func TestHandleFlowGrantToken_AllowedPathsAusente_SigueEmitiendo(t *testing.T) {
	h, fid := handlerConFlowActivo(t)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"flow_run_id": fid, "session_id": testSessionID}
	res, err := h.handleFlowGrantToken(context.Background(), req)
	require.NoError(t, err)

	require.False(t, res.IsError, "ausente = flow normal: no puede empezar a fallar")
}

func TestHandleFlowGrantToken_AllowedPathsNil_SigueEmitiendo(t *testing.T) {
	h, fid := handlerConFlowActivo(t)

	res := grantTokenCrudo(t, h, fid, nil)

	require.False(t, res.IsError, "un nil explícito es 'no declaré scope', no 'declaré y llegó roto'")
}
