package setup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// readClaudeJSON lee ~/.claude.json bajo el HOME del test.
func readClaudeJSON(t *testing.T, home string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc
}

// TestSetupClaudeCodeGlobal_CreatesFileWhenAbsent: si ~/.claude.json no
// existe, lo crea con mcpServers.domain (command/args/env).
func TestSetupClaudeCodeGlobal_CreatesFileWhenAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := SetupClaudeCodeGlobal("/usr/local/bin/domain-mcp", "sk_test", "http://localhost:8000")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".claude.json"), path)

	doc := readClaudeJSON(t, home)
	servers, ok := doc["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers debe existir")
	domain, ok := servers["domain"].(map[string]any)
	require.True(t, ok, "domain debe existir")
	require.Equal(t, "/usr/local/bin/domain-mcp", domain["command"])
	require.Equal(t, []any{}, domain["args"])

	env, ok := domain["env"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sk_test", env["DOMAIN_API_KEY"])
	require.Equal(t, "http://localhost:8000", env["DOMAIN_BASE_URL"])
}

// TestSetupClaudeCodeGlobal_PreservesOtherServers: merge — no pisa otros
// MCPs ni otras claves top-level del usuario.
func TestSetupClaudeCodeGlobal_PreservesOtherServers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pre := map[string]any{
		"someTopLevelKey": "preserved",
		"mcpServers": map[string]any{
			"engram": map[string]any{
				"command": "/home/u/.local/bin/memoria",
				"args":    []any{"mcp"},
			},
		},
	}
	raw, _ := json.MarshalIndent(pre, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), raw, 0o600))

	_, err := SetupClaudeCodeGlobal("/usr/local/bin/domain-mcp", "sk_test", "http://localhost:8000")
	require.NoError(t, err)

	doc := readClaudeJSON(t, home)
	require.Equal(t, "preserved", doc["someTopLevelKey"], "claves top-level se preservan")

	servers := doc["mcpServers"].(map[string]any)
	_, hasEngram := servers["engram"]
	require.True(t, hasEngram, "engram MCP se preserva")
	_, hasDomain := servers["domain"]
	require.True(t, hasDomain, "domain MCP se agrega")
}

// TestSetupClaudeCodeGlobal_Idempotent: segunda corrida no duplica y
// retorna ErrAlreadyConfigured.
func TestSetupClaudeCodeGlobal_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := SetupClaudeCodeGlobal("/usr/local/bin/domain-mcp", "sk_test", "http://localhost:8000")
	require.NoError(t, err)

	_, err = SetupClaudeCodeGlobal("/usr/local/bin/domain-mcp", "sk_test", "http://localhost:8000")
	require.True(t, errors.Is(err, ErrAlreadyConfigured), "segunda corrida → ErrAlreadyConfigured")

	doc := readClaudeJSON(t, home)
	servers := doc["mcpServers"].(map[string]any)
	require.Len(t, servers, 1, "domain no se duplica")
}
