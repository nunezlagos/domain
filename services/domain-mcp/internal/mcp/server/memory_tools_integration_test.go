//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/stretchr/testify/require"
)

// callToolRaw permite asertar tambien resultados de error del tool.
func callToolRaw(t *testing.T, srv *mcptest.Server, name string, args map[string]any) (string, bool) {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := srv.Client().CallTool(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	return tc.Text, result.IsError
}

func TestMCP_MemDelete(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	saveOut := callTool(t, f.srv, "domain_mem_save", map[string]any{
		"project_slug": f.projectSlug, "content": "para borrar",
	})
	var saved struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(saveOut), &saved))
	require.NotEmpty(t, saved.ID)

	out := callTool(t, f.srv, "domain_mem_delete", map[string]any{
		"observation_id": saved.ID,
	})
	var del struct {
		Deleted bool `json:"deleted"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &del))
	require.True(t, del.Deleted)

	// Doble delete → not found (soft-deleted)
	_, isErr := callToolRaw(t, f.srv, "domain_mem_delete", map[string]any{
		"observation_id": saved.ID,
	})
	require.True(t, isErr)

	// id inexistente → not found (anti-enumeration: mismo mensaje)
	_, isErr = callToolRaw(t, f.srv, "domain_mem_delete", map[string]any{
		"observation_id": "00000000-0000-0000-0000-000000000001",
	})
	require.True(t, isErr)
}

func TestMCP_MemSavePrompt_Versions(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	type promptRes struct {
		Version int  `json:"version"`
		Active  bool `json:"active"`
	}
	out1 := callTool(t, f.srv, "domain_mem_save_prompt", map[string]any{
		"slug": "review-pr", "body": "Revisa el PR {{pr_url}} con foco en seguridad",
	})
	var p1 promptRes
	require.NoError(t, json.Unmarshal([]byte(out1), &p1))
	require.Equal(t, 1, p1.Version)

	out2 := callTool(t, f.srv, "domain_mem_save_prompt", map[string]any{
		"slug": "review-pr", "body": "Revisa el PR {{pr_url}} con foco en seguridad y performance",
	})
	var p2 promptRes
	require.NoError(t, json.Unmarshal([]byte(out2), &p2))
	require.Equal(t, 2, p2.Version)
	require.True(t, p2.Active)

	// project inexistente → error claro
	_, isErr := callToolRaw(t, f.srv, "domain_mem_save_prompt", map[string]any{
		"slug": "x", "body": "y", "project_slug": "no-existe",
	})
	require.True(t, isErr)
}

func TestMCP_MemCapturePassive_Dedup(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	type capRes struct {
		Captured bool   `json:"captured"`
		Reason   string `json:"reason"`
	}
	out := callTool(t, f.srv, "domain_mem_capture_passive", map[string]any{
		"project_slug": f.projectSlug, "content": "contexto capturado pasivamente",
	})
	var c1 capRes
	require.NoError(t, json.Unmarshal([]byte(out), &c1))
	require.True(t, c1.Captured)

	// Mismo contenido → dedup (no error, captured false)
	out2 := callTool(t, f.srv, "domain_mem_capture_passive", map[string]any{
		"project_slug": f.projectSlug, "content": "contexto capturado pasivamente",
	})
	var c2 capRes
	require.NoError(t, json.Unmarshal([]byte(out2), &c2))
	require.False(t, c2.Captured)
	require.Equal(t, "duplicate", c2.Reason)
}

func TestMCP_MemSuggestTopicKey(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	out := callTool(t, f.srv, "domain_mem_suggest_topic_key", map[string]any{
		"content": "Migracion de postgres con pgvector: la migracion de postgres requiere extension pgvector",
	})
	var res struct {
		TopicKey string `json:"topic_key"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	require.Contains(t, res.TopicKey, "postgres")
	require.NotContains(t, res.TopicKey, " ")
}

func TestMCP_MemStats(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	_ = callTool(t, f.srv, "domain_mem_save", map[string]any{
		"project_slug": f.projectSlug, "content": "una decision importante",
		"observation_type": "decision",
	})
	_ = callTool(t, f.srv, "domain_mem_save", map[string]any{
		"project_slug": f.projectSlug, "content": "una nota",
	})

	out := callTool(t, f.srv, "domain_mem_stats", map[string]any{})
	var stats struct {
		Total  int64            `json:"observations_total"`
		ByType map[string]int64 `json:"observations_by_type"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &stats))
	require.EqualValues(t, 2, stats.Total)
	require.EqualValues(t, 1, stats.ByType["decision"])

	// Scoped a project
	out = callTool(t, f.srv, "domain_mem_stats", map[string]any{
		"project_slug": f.projectSlug,
	})
	var scoped struct {
		Total int64 `json:"observations_total"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &scoped))
	require.EqualValues(t, 2, scoped.Total)

	// project inexistente → error
	_, isErr := callToolRaw(t, f.srv, "domain_mem_stats", map[string]any{
		"project_slug": "nope",
	})
	require.True(t, isErr)
}
