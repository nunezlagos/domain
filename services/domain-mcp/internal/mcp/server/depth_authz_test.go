package mcpserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestAllowedToolsForDepth_DepthZero_NilFullAccess(t *testing.T) {
	require.Nil(t, AllowedToolsForDepth(Deps{}, 0), "depth 0 = full access (allowlist nil)")
	require.Nil(t, AllowedToolsForDepth(Deps{}, -1))
}

func TestAllowedToolsForDepth_DepthOne_ExcludesReentrantTools(t *testing.T) {
	allowed := AllowedToolsForDepth(Deps{}, 1)
	require.NotEmpty(t, allowed)

	set := map[string]bool{}
	for _, name := range allowed {
		set[name] = true
	}
	for tool := range reentrantTools {
		require.False(t, set[tool], "depth>=1 debe excluir la tool reentrante %s", tool)
	}
	require.True(t, set["domain_mem_search"], "mem_search debe seguir permitida en depth>=1")
}

func TestAllowedToolsForDepth_DepthOne_EnforcedByWrapper(t *testing.T) {
	allowed := AllowedToolsForDepth(Deps{}, 1)
	w := NewResilientWrapper(ToolBudget{})
	w.SetAllowedToolsAccessor(func() []string { return allowed })

	// las 3 reentrantes deben denegarse fail-closed
	for tool := range reentrantTools {
		res, err := w.Wrap(tool, authzStubHandler())(context.Background(), mcp.CallToolRequest{})
		require.NoError(t, err)
		require.True(t, res.IsError, "%s debe denegarse en un agente anidado (depth 1)", tool)
	}

	// mem_search debe pasar
	res, err := w.Wrap("domain_mem_search", authzStubHandler())(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, res.IsError, "mem_search debe seguir permitida")
}

func TestAllowedToolsForDepth_DenylistCubreSpawnYDiferidas(t *testing.T) {
	allowed := AllowedToolsForDepth(Deps{}, 1)
	set := map[string]bool{}
	for _, name := range allowed {
		set[name] = true
	}
	// tools que spawnean agentes o programan ejecución diferida deben quedar fuera
	for _, tool := range []string{
		"domain_cron_create",
		"domain_cron_set_enabled",
		"domain_orchestrate_phase_result",
		"domain_orchestrate_confirm",
		"domain_flow_create",
		"domain_skill_execute",
		"domain_agent_create",
	} {
		require.False(t, set[tool], "%s debe denegarse bajo depth>=1", tool)
	}
}

func TestAllowedToolsForDepthScoped_DepthZero_PreservaExisting(t *testing.T) {
	existing := []string{"domain_mem_search", "domain_agent_run"}
	require.Equal(t, existing, AllowedToolsForDepthScoped(Deps{}, 0, existing))
}

func TestAllowedToolsForDepthScoped_ExistingNil_UsaDepth(t *testing.T) {
	scoped := AllowedToolsForDepthScoped(Deps{}, 1, nil)
	require.ElementsMatch(t, AllowedToolsForDepth(Deps{}, 1), scoped)
}

func TestAllowedToolsForDepthScoped_Interseca_NuncaAmplia(t *testing.T) {
	// token ya scoped: incluye una reentrante y una segura + una inexistente
	existing := []string{"domain_mem_search", "domain_cron_create", "tool_inexistente"}
	scoped := AllowedToolsForDepthScoped(Deps{}, 1, existing)
	set := map[string]bool{}
	for _, n := range scoped {
		set[n] = true
	}
	require.True(t, set["domain_mem_search"], "la segura preexistente sobrevive")
	require.False(t, set["domain_cron_create"], "depth restringe la reentrante")
	require.False(t, set["tool_inexistente"], "depth no amplía a tools fuera del catálogo")
	require.LessOrEqual(t, len(scoped), len(existing), "intersección nunca amplía")
}
