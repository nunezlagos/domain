package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	orchsvc "nunezlagos/domain/internal/service/orchestrator"
	flowsvc "nunezlagos/domain/internal/service/flow"
)

// validateComoAgente valida el token declarando quién es el que valida. agentID vacío = hilo
// principal, que es como el hook llama hoy.
func validateComoAgente(t *testing.T, h *orchestrateHandlers, token, sessionID, agentID string) map[string]any {
	t.Helper()
	req := mcp.CallToolRequest{}
	args := map[string]any{"token": token, "session_id": sessionID}
	if agentID != "" {
		args["agent_id"] = agentID
	}
	req.Params.Arguments = args
	res, err := h.handleFlowValidateToken(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, res.Content)
	tc, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected text content")
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &m))
	return m
}

func handlerDeFlowActivo(t *testing.T) (*orchestrateHandlers, *flowsvc.FlowTokenService, uuid.UUID) {
	t.Helper()
	svc := flowsvc.NewFlowTokenService([]byte("test-secret-0123456789"))
	fid := uuid.New()
	h := newValidateHandler(&fakeOrch{statusResp: &orchsvc.FlowStatusResponse{FlowRunID: fid, Status: "running"}}, svc)
	return h, svc, fid
}

// DOMAINSERV-218 REQ-218.1. El session_id se HEREDA del padre, así que el check de sesión de
// DOMAINSERV-98 no discrimina entre dos subagentes de la misma sesión: sin este deny, A puede
// presentar el token de B y editar su territorio.
func TestHandleFlowValidateToken_TokenDeOtroAgente_ReturnsAgentMismatch(t *testing.T) {
	h, svc, fid := handlerDeFlowActivo(t)
	token, err := svc.GenerateTokenParaAgente(fid.String(), testSessionID, testOrgID, "agente-A", []string{"services/**"})
	require.NoError(t, err)

	res := validateComoAgente(t, h, token, testSessionID, "agente-B")

	require.Equal(t, false, res["valid"])
	require.Equal(t, "agent_mismatch", res["reason"])
	require.Nil(t, res["allowed_paths"], "un token denegado no puede filtrar el scope del otro agente")
}

// La simetría, sentido subagente → hilo principal.
func TestHandleFlowValidateToken_HiloPrincipalConTokenDeSubagente_ReturnsAgentMismatch(t *testing.T) {
	h, svc, fid := handlerDeFlowActivo(t)
	token, err := svc.GenerateTokenParaAgente(fid.String(), testSessionID, testOrgID, "agente-A", nil)
	require.NoError(t, err)

	res := validateComoAgente(t, h, token, testSessionID, "")

	require.Equal(t, false, res["valid"])
	require.Equal(t, "agent_mismatch", res["reason"])
}

// La simetría en el otro sentido, y es la que cierra la escalación: sin ella un subagente pide
// un token SIN agent_id y recupera la autorización amplia del flow.
func TestHandleFlowValidateToken_SubagenteConTokenSinAgente_ReturnsAgentMismatch(t *testing.T) {
	h, svc, fid := handlerDeFlowActivo(t)
	token, err := svc.GenerateToken(fid.String(), testSessionID, testOrgID)
	require.NoError(t, err)

	res := validateComoAgente(t, h, token, testSessionID, "agente-A")

	require.Equal(t, false, res["valid"])
	require.Equal(t, "agent_mismatch", res["reason"])
}

// REQ-218.6: el camino de hoy no se toca. El hook manda el token del hilo principal sin
// agent_id y tiene que seguir validando.
func TestHandleFlowValidateToken_HiloPrincipalConSuPropioToken_SigueValido(t *testing.T) {
	h, svc, fid := handlerDeFlowActivo(t)
	token, err := svc.GenerateToken(fid.String(), testSessionID, testOrgID)
	require.NoError(t, err)

	res := validateComoAgente(t, h, token, testSessionID, "")

	require.Equal(t, true, res["valid"])
}

func TestHandleFlowValidateToken_AgenteConSuPropioToken_EsValidoYDevuelveSuScope(t *testing.T) {
	h, svc, fid := handlerDeFlowActivo(t)
	token, err := svc.GenerateTokenParaAgente(fid.String(), testSessionID, testOrgID, "agente-A", []string{"services/**"})
	require.NoError(t, err)

	res := validateComoAgente(t, h, token, testSessionID, "agente-A")

	require.Equal(t, true, res["valid"])
	require.Equal(t, []any{"services/**"}, res["allowed_paths"])
}
