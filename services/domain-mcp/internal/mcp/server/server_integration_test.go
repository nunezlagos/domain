//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"strings"
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
	"nunezlagos/domain/internal/llm"
	mcpserver "nunezlagos/domain/internal/mcp/server"
	dmigrate "nunezlagos/domain/internal/migrate"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	agentsvc "nunezlagos/domain/internal/service/agent"
	cronsvc "nunezlagos/domain/internal/service/cron"
	flowsvc "nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/observation"
	policysvc "nunezlagos/domain/internal/service/policy"
	projsvc "nunezlagos/domain/internal/service/project"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

type mcpFixture struct {
	srv         *mcptest.Server
	projectSlug string
	skills      *skillsvc.Service
	policies    *policysvc.Service
	orgID       uuid.UUID
	userID      uuid.UUID
	cleanup     func()
}

func setupMCP(t *testing.T) *mcpFixture {
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
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}
	obsS := &observation.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}

	org, owner, err := seedOrgUser(ctx, pools.App, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)
	proj, err := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})
	require.NoError(t, err)

	skillS := &skillsvc.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	policyS := &policysvc.Service{Pool: pools.App}
	deps := mcpserver.Deps{
		Observations: obsS,
		Projects:     projS,
		Prompts:      &promptsvc.Service{Pool: pools.App, Audit: rec},
		Skills:       skillS,
		SkillExecution: &skillsvc.ExecutionService{
			Pool: pools.App, Skills: skillS,
			Versions: &skillsvc.VersionStore{Pool: pools.App},
			Runner:   skillrunner.New(),
		},
		Agents:   &agentsvc.Service{Pool: pools.App, Audit: rec},
		Flows:    &flowsvc.Service{Pool: pools.App, Audit: rec},
		Crons:    &cronsvc.Service{Pool: pools.App, Audit: rec},
		Policies: policyS,
		Pool:     pools.App,
		Principal: &apikey.Principal{
			UserID:         owner.UserID.String(),
			OrganizationID: org.ID.String(),
			Role:           "owner",
		},
		ServerName: "domain-mcp-test",
		ServerVer:  "0.0.0",
	}

	testSrv, err := mcptest.NewServer(t, mcpserver.Tools(deps)...)
	require.NoError(t, err)

	return &mcpFixture{
		srv:         testSrv,
		projectSlug: proj.Slug,
		skills:      skillS,
		policies:    policyS,
		orgID:       org.ID,
		userID:      owner.UserID,
		cleanup: func() {
			testSrv.Close()
			pools.Close()
			_ = pgC.Terminate(ctx)
		},
	}
}

func callTool(t *testing.T, srv *mcptest.Server, name string, args map[string]any) string {
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

func TestMCP_MemSave_AndContext(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	saveOut := callTool(t, f.srv, "domain_mem_save", map[string]any{
		"project_slug": f.projectSlug,
		"content":      "decidimos usar pgvector con embeddings",
		"tags":         []any{"arch"},
	})
	require.Contains(t, saveOut, "observation saved")

	ctxOut := callTool(t, f.srv, "domain_mem_context", map[string]any{
		"project_slug": f.projectSlug,
		"limit":        float64(10),
	})
	require.Contains(t, ctxOut, "pgvector")
}

func TestMCP_MemSearch_HybridFindsMatch(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	for _, c := range []string{
		"decidimos usar pgvector con embeddings de openai",
		"el clima en santiago esta soleado",
		"pgvector soporta busqueda hibrida con ivfflat",
	} {
		_ = callTool(t, f.srv, "domain_mem_save", map[string]any{
			"project_slug": f.projectSlug,
			"content":      c,
		})
	}

	searchOut := callTool(t, f.srv, "domain_mem_search", map[string]any{
		"query": "pgvector embeddings",
		"limit": float64(5),
	})
	var resp struct {
		Results []map[string]any `json:"results"`
		Count   int              `json:"count"`
	}
	require.NoError(t, json.Unmarshal([]byte(searchOut), &resp))
	require.NotEmpty(t, resp.Results, "search debe devolver al menos un resultado")
	top := resp.Results[0]["content"].(string)
	require.True(t, strings.Contains(strings.ToLower(top), "pgvector"),
		"top result debe ser sobre pgvector, no clima")
}

func TestMCP_MemGetObservation_RoundTrip(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	saveOut := callTool(t, f.srv, "domain_mem_save", map[string]any{
		"project_slug": f.projectSlug,
		"content":      "observacion especifica para round-trip",
	})
	var saveResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(saveOut), &saveResp))

	getOut := callTool(t, f.srv, "domain_mem_get_observation", map[string]any{
		"id": saveResp.ID,
	})
	require.Contains(t, getOut, "round-trip")
}

// Sabotaje: get con UUID valido pero de otra org → not found (cross-org guard).
func TestSabotage_MCP_CrossOrgGetReturnsNotFound(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "domain_mem_get_observation"
	req.Params.Arguments = map[string]any{"id": uuid.New().String()}
	result, err := f.srv.Client().CallTool(ctx, req)
	require.NoError(t, err)
	require.True(t, result.IsError, "UUID inexistente debe devolver IsError=true")
}

// Semantica nueva (plug-and-play): project_slug inexistente se
// AUTO-CREA en mem_save — en un install fresco no hay projects y el
// agente debe poder guardar memoria igual. El sabotaje pasa a ser:
// slug INVALIDO (no slugificable) → error limpio, sin panic ni project
// basura.
func TestMCP_MemSave_AutoCreatesProject(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	out := callTool(t, f.srv, "domain_mem_save", map[string]any{
		"project_slug": "proyecto-nuevo",
		"content":      "primera memoria del proyecto nuevo",
	})
	require.Contains(t, out, "id", "save con project nuevo debe funcionar (auto-create)")

	listOut := callTool(t, f.srv, "domain_project_list", map[string]any{})
	require.Contains(t, listOut, "proyecto-nuevo")

	_ = ctx
}

// Sabotaje: slug invalido (mayusculas/espacios) → el auto-create debe
// rechazarlo con error de tool, no crear un project malformado.
func TestSabotage_MCP_InvalidProjectSlug(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "domain_mem_save"
	req.Params.Arguments = map[string]any{
		"project_slug": "NO Valido!!",
		"content":      "x",
	}
	result, err := f.srv.Client().CallTool(ctx, req)
	require.NoError(t, err)
	require.True(t, result.IsError, "slug invalido debe fallar limpio")
}
