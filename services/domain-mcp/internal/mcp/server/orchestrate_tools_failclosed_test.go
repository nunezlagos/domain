package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/apikey"
	flowsvc "nunezlagos/domain/internal/service/flow"
	orchsvc "nunezlagos/domain/internal/service/orchestrator"
)

// fakeOrch implementa orchestratorService; solo GetFlowStatus es relevante.
type fakeOrch struct {
	statusResp *orchsvc.FlowStatusResponse
	statusErr  error
}

func (f *fakeOrch) Run(context.Context, orchsvc.OrchestrateInput) (*orchsvc.OrchestrateResult, error) {
	return nil, nil
}
func (f *fakeOrch) RecordPhaseResult(context.Context, orchsvc.PhaseResultInput) (*orchsvc.PhaseResultResult, error) {
	return nil, nil
}
func (f *fakeOrch) ConfirmContinue(context.Context, uuid.UUID, bool) (*orchsvc.PhaseResultResult, error) {
	return nil, nil
}
func (f *fakeOrch) GetFlowStatus(context.Context, uuid.UUID) (*orchsvc.FlowStatusResponse, error) {
	return f.statusResp, f.statusErr
}
func (f *fakeOrch) CancelFlow(context.Context, uuid.UUID, string) (*orchsvc.FlowStatusResponse, error) {
	return nil, nil
}

const (
	testOrgID     = "11111111-1111-1111-1111-111111111111"
	testSessionID = "sess-1"
)

func validateTokenResult(t *testing.T, h *orchestrateHandlers, token, sessionID string) map[string]any {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"token": token, "session_id": sessionID}
	res, err := h.handleFlowValidateToken(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, res.Content)
	tc, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected text content")
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &m))
	return m
}

func newValidateHandler(orch orchestratorService, svc *flowsvc.FlowTokenService) *orchestrateHandlers {
	return &orchestrateHandlers{
		orchestrator: orch,
		principal:    &apikey.Principal{OrganizationID: testOrgID, UserID: uuid.NewString(), Role: "owner"},
		flowToken:    svc,
	}
}

// DOMAINSERV-94: token firmado y no expirado cuyo flow no existe (GetFlowStatus
// devuelve error) debe validar como INVÁLIDO — fail-closed, sin pase libre.
func TestHandleFlowValidateToken_FlowNotFound_ReturnsInvalid(t *testing.T) {
	svc := flowsvc.NewFlowTokenService([]byte("test-secret-0123456789"))
	token, err := svc.GenerateToken(uuid.NewString(), testSessionID, testOrgID)
	require.NoError(t, err)

	h := newValidateHandler(&fakeOrch{statusErr: errors.New("flow_run not found")}, svc)
	res := validateTokenResult(t, h, token, testSessionID)

	require.Equal(t, false, res["valid"])
	require.Equal(t, "flow_inactive", res["reason"])
}

// Camino positivo: flow en running valida true (que el fail-closed no rompa el caso legítimo).
func TestHandleFlowValidateToken_RunningFlow_ReturnsValid(t *testing.T) {
	svc := flowsvc.NewFlowTokenService([]byte("test-secret-0123456789"))
	fid := uuid.New()
	token, err := svc.GenerateToken(fid.String(), testSessionID, testOrgID)
	require.NoError(t, err)

	h := newValidateHandler(&fakeOrch{statusResp: &orchsvc.FlowStatusResponse{FlowRunID: fid, Status: "running"}}, svc)
	res := validateTokenResult(t, h, token, testSessionID)

	require.Equal(t, true, res["valid"])
}

// DOMAINSERV-98: token de otra org → org_mismatch (corta replay cross-tenant).
func TestHandleFlowValidateToken_OrgMismatch_ReturnsInvalid(t *testing.T) {
	svc := flowsvc.NewFlowTokenService([]byte("test-secret-0123456789"))
	fid := uuid.New()
	token, err := svc.GenerateToken(fid.String(), testSessionID, "99999999-9999-9999-9999-999999999999")
	require.NoError(t, err)

	h := newValidateHandler(&fakeOrch{statusResp: &orchsvc.FlowStatusResponse{FlowRunID: fid, Status: "running"}}, svc)
	res := validateTokenResult(t, h, token, testSessionID)

	require.Equal(t, false, res["valid"])
	require.Equal(t, "org_mismatch", res["reason"])
}

// DOMAINSERV-98: token de otra sesión → session_mismatch (corta replay cross-sesión).
func TestHandleFlowValidateToken_SessionMismatch_ReturnsInvalid(t *testing.T) {
	svc := flowsvc.NewFlowTokenService([]byte("test-secret-0123456789"))
	fid := uuid.New()
	token, err := svc.GenerateToken(fid.String(), "otra-sesion", testOrgID)
	require.NoError(t, err)

	h := newValidateHandler(&fakeOrch{statusResp: &orchsvc.FlowStatusResponse{FlowRunID: fid, Status: "running"}}, svc)
	res := validateTokenResult(t, h, token, testSessionID)

	require.Equal(t, false, res["valid"])
	require.Equal(t, "session_mismatch", res["reason"])
}
