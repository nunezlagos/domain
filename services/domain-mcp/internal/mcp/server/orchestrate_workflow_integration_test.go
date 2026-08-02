//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/db"
	mcpserver "nunezlagos/domain/internal/mcp/server"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/observability"
	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// setupWorkflowMCP levanta el MCP con el tracker de workflows real detras del
// hook de metricas: esa es la unica via por la que una tool call llega a la
// tabla `workflows` en produccion.
func setupWorkflowMCP(t *testing.T) (*mcptest.Server, string, *observability.PGWorkflowStore, func()) {
	t.Helper()
	ctx := context.Background()
	pools, terminar := pgMigradoParaWorkflows(t, ctx)
	org, owner, err := seedOrgUser(ctx, pools.App, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)
	_, err = seeds.SeedAgentTemplatesForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)
	var projectID string
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('Demo', 'demo') RETURNING id`).Scan(&projectID))

	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	store := &observability.PGWorkflowStore{Pool: pools.App}
	tracker := observability.NewTracker(store, nil, 0, 0)

	srv, err := mcptest.NewServer(t, mcpserver.Tools(mcpserver.Deps{
		Principal: &apikey.Principal{
			UserID:         owner.UserID.String(),
			OrganizationID: org.ID.String(),
			Role:           "owner",
		},
		Orchestrator: orchestrator.New(pools.App, &audit.PGRecorder{Pool: pools.Auth}, reg, "dev"),
		ServerName:   "domain-mcp-test",
		ServerVer:    "0.0.0",
		MetricsOnToolCall: func(ctx context.Context, tool, status, errCode, errMsg string, dur float64) {
			observability.LogToolInvocation(ctx, nil, nil, tracker, observability.ToolCall{
				Tool: tool, Status: status, ErrorCode: errCode, ErrorMessage: errMsg,
				DurationMS: int64(dur * 1000),
			})
		},
	})...)
	require.NoError(t, err)

	return srv, projectID, store, func() {
		srv.Close()
		pools.Close()
		terminar()
	}
}

func pgMigradoParaWorkflows(t *testing.T, ctx context.Context) (*db.Pools, func()) {
	t.Helper()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	return pools, func() { _ = pgC.Terminate(ctx) }
}

// DOMAINSERV-212: la corrida del orquestador es el productor de workflow_id.
// Dos tool calls que declaran la misma corrida tienen que acumularse en UNA
// fila con su nombre y su cuenta, no en N filas de una.
func TestMCP_FlowRun_DosToolCalls_AcumulanEnUnaFilaDeWorkflow(t *testing.T) {
	srv, projectID, store, cleanup := setupWorkflowMCP(t)
	defer cleanup()
	ctx := context.Background()

	startTxt := callOrchTool(t, srv, "domain_orchestrate", map[string]any{
		"raw_text":   "fix typo en README",
		"mode":       "express",
		"project_id": projectID,
	})
	var start struct {
		FlowRunID string `json:"flow_run_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(startTxt), &start))
	flowRunID := uuid.MustParse(start.FlowRunID)

	callOrchTool(t, srv, "domain_flow_status", map[string]any{"flow_run_id": start.FlowRunID})
	callOrchTool(t, srv, "domain_flow_status", map[string]any{"flow_run_id": start.FlowRunID})

	w, err := store.GetWorkflow(ctx, flowRunID)
	require.NoError(t, err)
	require.Equal(t, observability.WorkflowNameFlowRun, w.Name)
	require.Equal(t, 2, w.TotalToolCalls)
	require.Equal(t, 0, w.TotalErrors)
	require.Equal(t, observability.WorkflowRunning, w.Status)

	var filas int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM workflows`).Scan(&filas))
	require.Equal(t, 1, filas, "domain_orchestrate no declara corrida: no debe abrir una fila propia")
}
