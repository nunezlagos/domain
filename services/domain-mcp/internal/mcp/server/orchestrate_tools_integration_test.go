//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
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
	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

type orchFixture struct {
	srv       *mcptest.Server
	projectID string
	cleanup   func()
}

func setupOrchMCP(t *testing.T) *orchFixture {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	rec := &audit.PGRecorder{Pool: pools.Auth}
	org, owner, err := seedOrgUser(ctx, pools.App, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)
	_, err = seeds.SeedAgentTemplatesForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)

	// domain_orchestrate exige project_id (flow_runs.project_id NOT NULL): un
	// project real al que scopear el run.
	var projectID string
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('Demo', 'demo') RETURNING id`,
	).Scan(&projectID))

	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	orchSvc := orchestrator.New(pools.App, rec, reg, "dev")

	deps := mcpserver.Deps{
		Principal: &apikey.Principal{
			UserID:         owner.UserID.String(),
			OrganizationID: org.ID.String(),
			Role:           "owner",
		},
		Orchestrator: orchSvc,
		ServerName:   "domain-mcp-test",
		ServerVer:    "0.0.0",
	}
	testSrv, err := mcptest.NewServer(t, mcpserver.Tools(deps)...)
	require.NoError(t, err)

	return &orchFixture{
		srv:       testSrv,
		projectID: projectID,
		cleanup: func() {
			testSrv.Close()
			pools.Close()
			_ = pgC.Terminate(ctx)
		},
	}
}

func callOrchTool(t *testing.T, srv *mcptest.Server, name string, args map[string]any) string {
	t.Helper()
	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := srv.Client().CallTool(ctx, req)
	require.NoError(t, err)
	require.Falsef(t, result.IsError, "tool '%s' error: %+v", name, result.Content)
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected text content")
	return tc.Text
}

func TestMCP_Orchestrate_Express_RoundTrip(t *testing.T) {
	f := setupOrchMCP(t)
	defer f.cleanup()

	startTxt := callOrchTool(t, f.srv, "domain_orchestrate", map[string]any{
		"raw_text":   "fix typo en README",
		"mode":       "express",
		"project_id": f.projectID,
	})
	var startRes struct {
		OrchestratorRunID string `json:"OrchestratorRunID"`
		FlowRunID         string `json:"FlowRunID"`
		Mode              string `json:"Mode"`
		Plan              struct {
			Mode  string `json:"Mode"`
			Steps []struct {
				ID                string `json:"ID"`
				Slug              string `json:"Slug"`
				AgentTemplateSlug string `json:"AgentTemplateSlug"`
				SystemPrompt      string `json:"SystemPrompt"`
				UserPrompt        string `json:"UserPrompt"`
				SuggestedSaves    []struct {
					Type     string `json:"Type"`
					Required bool   `json:"Required"`
				} `json:"SuggestedSaves"`
			} `json:"Steps"`
		} `json:"Plan"`
	}
	require.NoError(t, json.Unmarshal([]byte(startTxt), &startRes))
	require.Equal(t, "express", startRes.Mode)
	require.Len(t, startRes.Plan.Steps, 2)
	require.Equal(t, "sdd-apply", startRes.Plan.Steps[0].Slug)
	require.Equal(t, "sdd-verify", startRes.Plan.Steps[1].Slug)

	applyStepID := startRes.Plan.Steps[0].ID
	verifyStepID := startRes.Plan.Steps[1].ID

	applyTxt := callOrchTool(t, f.srv, "domain_orchestrate_phase_result", map[string]any{
		"flow_run_step_id": applyStepID,
		"output": map[string]any{
			"files_changed": []any{"README.md"},
			"summary":       "typo fix",
		},
		"memory_refs_saved": []any{
			map[string]any{"type": "code_reference", "id": uuid.New().String()},
		},
	})
	var applyRes struct {
		StepStatus    string `json:"StepStatus"`
		FlowRunStatus string `json:"FlowRunStatus"`
		NextStepKey   string `json:"NextStepKey"`
	}
	require.NoError(t, json.Unmarshal([]byte(applyTxt), &applyRes))
	require.Equal(t, "completed", applyRes.StepStatus)
	require.Equal(t, "running", applyRes.FlowRunStatus)
	require.Equal(t, "sdd-verify", applyRes.NextStepKey)

	verifyTxt := callOrchTool(t, f.srv, "domain_orchestrate_phase_result", map[string]any{
		"flow_run_step_id": verifyStepID,
		"output": map[string]any{
			"scenarios_failed": []any{},
			"tests_passed":     1,
		},
		// La fase sdd-verify declara required_tool_calls (REQ-54): sin reportarlas
		// el orquestador deja el step running (reintentable) y no lo cierra.
		"tool_calls": []any{"domain_verify_start", "domain_verify_complete"},
	})
	var verifyRes struct {
		StepStatus    string `json:"StepStatus"`
		FlowRunStatus string `json:"FlowRunStatus"`
	}
	require.NoError(t, json.Unmarshal([]byte(verifyTxt), &verifyRes))
	require.Equal(t, "completed", verifyRes.StepStatus)
	require.Equal(t, "completed", verifyRes.FlowRunStatus)

	statusTxt := callOrchTool(t, f.srv, "domain_flow_status", map[string]any{
		"flow_run_id": startRes.FlowRunID,
	})
	var statusRes struct {
		Status string `json:"status"`
		Mode   string `json:"mode"`
		Steps  []struct {
			Status  string `json:"status"`
			StepKey string `json:"step_key"`
		} `json:"steps"`
	}
	require.NoError(t, json.Unmarshal([]byte(statusTxt), &statusRes))
	require.Equal(t, "completed", statusRes.Status)
	require.Equal(t, "express", statusRes.Mode)
	require.Len(t, statusRes.Steps, 2)
	for _, s := range statusRes.Steps {
		require.Equal(t, "completed", s.Status)
	}
}

// Sin Orchestrator inyectado → cualquier llamada al tool devuelve error
// "orchestrator service not configured", NO crash, NO 500.
func TestMCP_Orchestrate_NotConfigured_ReturnsError(t *testing.T) {
	ctx := context.Background()
	deps := mcpserver.Deps{
		Principal: &apikey.Principal{
			UserID:         uuid.New().String(),
			OrganizationID: uuid.New().String(),
			Role:           "owner",
		},

		ServerName: "test", ServerVer: "0.0.0",
	}
	srv, err := mcptest.NewServer(t, mcpserver.Tools(deps)...)
	require.NoError(t, err)
	defer srv.Close()

	req := mcp.CallToolRequest{}
	req.Params.Name = "domain_orchestrate"
	req.Params.Arguments = map[string]any{"raw_text": "x", "mode": "express"}
	result, err := srv.Client().CallTool(ctx, req)
	require.NoError(t, err)
	require.True(t, result.IsError, "debe devolver error con Orchestrator nil")
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, "orchestrator service not configured")
}
