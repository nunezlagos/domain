package mcpserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

func authzStubHandler() mcpgo.ToolHandlerFunc {
	return func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	}
}

func TestResilientWrapper_Wrap_ToolNotInAllowlist_Denied(t *testing.T) {
	w := NewResilientWrapper(ToolBudget{})
	w.SetAllowedToolsAccessor(func() []string { return []string{"domain_mem_search"} })
	h := w.Wrap("domain_agent_run", authzStubHandler())
	res, err := h(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.IsError, "tool fuera del allowlist debe denegarse (anti-reentrancia)")
}

func TestResilientWrapper_Wrap_ToolInAllowlist_Allowed(t *testing.T) {
	w := NewResilientWrapper(ToolBudget{})
	w.SetAllowedToolsAccessor(func() []string { return []string{"domain_mem_search"} })
	h := w.Wrap("domain_mem_search", authzStubHandler())
	res, err := h(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, res.IsError)
}

func TestResilientWrapper_Wrap_NilAllowlist_AllowsAll(t *testing.T) {
	w := NewResilientWrapper(ToolBudget{})
	h := w.Wrap("domain_agent_run", authzStubHandler())
	res, err := h(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, res.IsError, "allowlist nil/vacío = full access (backward-compat)")
}

func TestResilientWrapper_Wrap_EmptyAllowlist_AllowsAll(t *testing.T) {
	w := NewResilientWrapper(ToolBudget{})
	w.SetAllowedToolsAccessor(func() []string { return []string{} })
	h := w.Wrap("domain_agent_run", authzStubHandler())
	res, err := h(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, res.IsError, "allowlist vacío = full access")
}
