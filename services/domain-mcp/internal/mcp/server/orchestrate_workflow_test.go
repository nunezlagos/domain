package mcpserver

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/observability"
)

// workflowsVistos invoca un tool del orquestador `veces` veces y devuelve el
// workflow_id que vio el hook de metricas en cada llamada. Ese hook es el ctx
// del que LogToolInvocation lee el workflow para tocar la tabla `workflows`,
// asi que es el unico punto de observacion que representa produccion.
func workflowsVistos(t *testing.T, tool string, args map[string]any, veces int) []uuid.UUID {
	t.Helper()
	var vistos []uuid.UUID
	wrap := NewResilientWrapper(defaultBudget)
	wrap.SetMetricsHooks(func(ctx context.Context, _, _, _, _ string, _ float64) {
		vistos = append(vistos, observability.WorkflowIDFromContext(ctx))
	}, nil, nil)
	tools := registerOrchestrateTools(wrap, Deps{Principal: &apikey.Principal{
		UserID:         uuid.New().String(),
		OrganizationID: uuid.New().String(),
	}})
	var handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	for _, st := range tools {
		if st.Tool.Name == tool {
			handler = st.Handler
		}
	}
	require.NotNil(t, handler, "tool %s no registrada", tool)
	for i := 0; i < veces; i++ {
		req := mcp.CallToolRequest{}
		req.Params.Name = tool
		req.Params.Arguments = args
		_, err := handler(context.Background(), req)
		require.NoError(t, err)
	}
	return vistos
}

func TestOrchestrateTools_FlowStatus_DosInvocacionesDelMismoFlow_CompartenWorkflowID(t *testing.T) {
	flowRunID := uuid.New()
	vistos := workflowsVistos(t, "domain_flow_status",
		map[string]any{"flow_run_id": flowRunID.String()}, 2)
	require.Len(t, vistos, 2)
	require.Equal(t, flowRunID, vistos[0], "el workflow de la corrida es su flow_run_id")
	require.Equal(t, vistos[0], vistos[1], "dos tool calls del mismo flow comparten workflow")
}

// invariante de DOMAINSERV-189: sin corrida declarada no se inventa un id, que
// era la fabrica de una fila basura por invocacion
func TestOrchestrateTools_ValidateToken_SinFlowRunID_NoInventaWorkflow(t *testing.T) {
	vistos := workflowsVistos(t, "domain_flow_validate_token",
		map[string]any{"token": "x", "session_id": "s"}, 1)
	require.Equal(t, []uuid.UUID{uuid.Nil}, vistos)
}
