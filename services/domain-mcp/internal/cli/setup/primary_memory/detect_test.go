package primary_memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestDetect_OpenCode_FindsEngram(t *testing.T) {
	cfg := `{
		"$schema": "https://opencode.ai/config.json",
		"mcp": {
			"engram": {"command": ["/usr/bin/engram"], "enabled": true},
			"domain": {"command": ["/usr/bin/domain-mcp"], "enabled": true},
			"filesystem": {"command": ["/usr/bin/fs-mcp"]}
		}
	}`
	path := writeConfig(t, cfg)
	got, err := Detect("opencode", path)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Construir map para asssert rápido.
	byName := map[string]DetectedProvider{}
	for _, p := range got {
		byName[p.Name] = p
	}
	require.True(t, byName["engram"].IsMemory, "engram debe ser memory")
	require.True(t, byName["filesystem"].IsMemory == false, "filesystem no es memory")
	require.False(t, byName["domain"].IsMemory, "domain no es memory (es nuestro)")
	require.Equal(t, "opencode", byName["engram"].Agent)
}

func TestDetect_OpenCode_IgnoresFilesystem(t *testing.T) {
	cfg := `{
		"mcp": {
			"filesystem": {"command": ["/usr/bin/fs-mcp"]},
			"github": {"command": ["/usr/bin/gh-mcp"]},
			"domain": {"command": ["/usr/bin/domain-mcp"]}
		}
	}`
	path := writeConfig(t, cfg)
	got, err := Detect("opencode", path)
	require.NoError(t, err)
	mem := MemoryProviders(got)
	require.Empty(t, mem, "ninguno de filesystem/github/domain es memory provider")
}

func TestDetect_ClaudeCode_FindsMem0(t *testing.T) {
	cfg := `{
		"mcpServers": {
			"mem0": {"command": "/usr/bin/mem0"},
			"domain": {"command": "/usr/bin/domain-mcp"}
		}
	}`
	path := writeConfig(t, cfg)
	got, err := Detect("claude-code", path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byName := map[string]DetectedProvider{}
	for _, p := range got {
		byName[p.Name] = p
	}
	require.True(t, byName["mem0"].IsMemory)
	require.False(t, byName["domain"].IsMemory)
}

func TestDetect_NoMemoryProviders(t *testing.T) {
	cfg := `{"mcp": {"domain": {"command": ["/domain"]}}}`
	path := writeConfig(t, cfg)
	got, err := Detect("opencode", path)
	require.NoError(t, err)
	mem := MemoryProviders(got)
	require.Empty(t, mem, "solo domain → no hay otros memory providers")
}

func TestDetect_MissingConfig_ReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	got, err := Detect("opencode", filepath.Join(tmp, "nope.json"))
	require.NoError(t, err, "missing config no es error")
	require.Empty(t, got)
}

func TestDetect_MalformedJSON_ReturnsEmpty(t *testing.T) {
	// JSON inválido: warning + lista vacía. No fallamos.
	path := writeConfig(t, `{"mcp": invalid json {{{`)
	got, err := Detect("opencode", path)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestDetect_UnknownAgent_ReturnsError(t *testing.T) {
	path := writeConfig(t, `{}`)
	_, err := Detect("cursor", path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown agent")
}

func TestIsAlreadyDisabled_OpenCode_CommandFalse(t *testing.T) {
	cfg := `{
		"mcp": {
			"engram": {"command": false, "enabled": false}
		}
	}`
	path := writeConfig(t, cfg)
	disabled, err := IsAlreadyDisabled("opencode", path, "engram")
	require.NoError(t, err)
	require.True(t, disabled)
}

func TestIsAlreadyDisabled_OpenCode_CommandEmptyArray(t *testing.T) {
	cfg := `{"mcp": {"engram": {"command": []}}}`
	path := writeConfig(t, cfg)
	disabled, err := IsAlreadyDisabled("opencode", path, "engram")
	require.NoError(t, err)
	require.True(t, disabled, "command=[] también es 'disabled'")
}

func TestIsAlreadyDisabled_OpenCode_EnabledTrue_NotDisabled(t *testing.T) {
	cfg := `{"mcp": {"engram": {"command": ["/engram"], "enabled": true}}}`
	path := writeConfig(t, cfg)
	disabled, err := IsAlreadyDisabled("opencode", path, "engram")
	require.NoError(t, err)
	require.False(t, disabled)
}

func TestMemoryProviders_FiltersCorrectly(t *testing.T) {
	all := []DetectedProvider{
		{Name: "engram", IsMemory: true},
		{Name: "mem0", IsMemory: true},
		{Name: "filesystem", IsMemory: false},
		{Name: "domain", IsMemory: false},
	}
	mem := MemoryProviders(all)
	require.Len(t, mem, 2)
	require.ElementsMatch(t, []string{"engram", "mem0"}, SortedNames(mem))
}

func TestDetect_PreservesValidJSON(t *testing.T) {
	// Después del detect, el archivo original debe quedar intacto.
	cfg := `{"mcp": {"engram": {"command": ["/engram"]}, "domain": {"command": ["/domain"]}}}`
	path := writeConfig(t, cfg)
	_, err := Detect("opencode", path)
	require.NoError(t, err)
	body, _ := os.ReadFile(path)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	require.Contains(t, doc, "mcp")
}
