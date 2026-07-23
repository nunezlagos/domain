//go:build integration

package mcpserver_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-93 A: proyecto vacío se borra sin force.
func TestMCP_ProjectDelete_EmptyProject_Deletes(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	out := callTool(t, f.srv, "domain_project_delete", map[string]any{
		"slug": f.projectSlug,
	})
	var res struct {
		Deleted bool `json:"deleted"`
		HadData bool `json:"had_data"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	require.True(t, res.Deleted)
	require.False(t, res.HadData)
}

// DOMAINSERV-93 A: proyecto con datos → rechazado salvo force=true.
func TestMCP_ProjectDelete_WithData_RejectedUnlessForce(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	_ = callTool(t, f.srv, "domain_mem_save", map[string]any{
		"project_slug": f.projectSlug,
		"content":      "una observación que hace al proyecto no-vacío",
	})

	// sin force → error del guard
	raw, isErr := callToolRaw(t, f.srv, "domain_project_delete", map[string]any{
		"slug": f.projectSlug,
	})
	require.True(t, isErr, "borrar un proyecto con datos sin force debe fallar")
	require.Contains(t, strings.ToLower(raw), "force")

	// con force → borra
	out := callTool(t, f.srv, "domain_project_delete", map[string]any{
		"slug":  f.projectSlug,
		"force": true,
	})
	var res struct {
		Deleted bool `json:"deleted"`
		HadData bool `json:"had_data"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	require.True(t, res.Deleted)
	require.True(t, res.HadData)
}
