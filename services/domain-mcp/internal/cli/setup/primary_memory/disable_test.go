package primary_memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// helper: escribir un config con un entry específico.
func writeConfigWithEntry(t *testing.T, agent string, name string, entry map[string]any) string {
	t.Helper()
	tmp := t.TempDir()
	var path string
	if agent == "opencode" {
		path = filepath.Join(tmp, "opencode.json")
		doc := map[string]any{
			"mcp": map[string]any{name: entry},
		}
		body, _ := json.MarshalIndent(doc, "", "  ")
		require.NoError(t, os.WriteFile(path, body, 0o600))
	} else {
		path = filepath.Join(tmp, ".claude.json")
		doc := map[string]any{
			"mcpServers": map[string]any{name: entry},
		}
		body, _ := json.MarshalIndent(doc, "", "  ")
		require.NoError(t, os.WriteFile(path, body, 0o600))
	}
	return path
}

func TestDisable_OpenCode_SetsCommandFalse(t *testing.T) {
	entry := map[string]any{
		"command": []any{"/usr/bin/engram"},
		"enabled": true,
		"type":    "local",
	}
	path := writeConfigWithEntry(t, "opencode", "engram", entry)

	err := Disable("opencode", path, []string{"engram"})
	require.NoError(t, err)

	body, _ := os.ReadFile(path)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	mcps := doc["mcp"].(map[string]any)
	engram := mcps["engram"].(map[string]any)

	require.Equal(t, false, engram["command"], "command debe ser false")

	require.Equal(t, "local", engram["type"])
	require.Equal(t, true, engram["enabled"], "enabled se preserva")
}

func TestDisable_ClaudeCode_SetsCommandFalse(t *testing.T) {
	entry := map[string]any{
		"command": "/usr/bin/mem0",
		"env":     map[string]any{"MEM0_KEY": "abc"},
	}
	path := writeConfigWithEntry(t, "claude-code", "mem0", entry)

	err := Disable("claude-code", path, []string{"mem0"})
	require.NoError(t, err)

	body, _ := os.ReadFile(path)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	servers := doc["mcpServers"].(map[string]any)
	mem0 := servers["mem0"].(map[string]any)
	require.Equal(t, false, mem0["command"])

	env, _ := mem0["env"].(map[string]any)
	require.Equal(t, "abc", env["MEM0_KEY"])
}

func TestDisable_BackupCreated(t *testing.T) {
	entry := map[string]any{"command": []any{"/engram"}}
	path := writeConfigWithEntry(t, "opencode", "engram", entry)
	originalBody, _ := os.ReadFile(path)


	err := Disable("opencode", path, []string{"engram"})
	require.NoError(t, err)


	dir := filepath.Dir(path)
	pattern := filepath.Base(path) + ".bak.*"
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	require.NoError(t, err)
	require.NotEmpty(t, matches, "debe existir un backup .bak.*, dir=%v", dir)


	bakBody, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	require.Equal(t, originalBody, bakBody, "backup debe tener contenido original")


	bakName := filepath.Base(matches[0])
	require.Regexp(t, `\.bak\.\d{8}T\d{6}Z$`, bakName)
}

func TestDisable_MultipleProviders(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "opencode.json")
	doc := map[string]any{
		"mcp": map[string]any{
			"engram":   map[string]any{"command": []any{"/engram"}},
			"mem0":     map[string]any{"command": []any{"/mem0"}},
			"domain":   map[string]any{"command": []any{"/domain"}},
			"filesystem": map[string]any{"command": []any{"/fs"}},
		},
	}
	body, _ := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, os.WriteFile(path, body, 0o600))

	err := Disable("opencode", path, []string{"engram", "mem0"})
	require.NoError(t, err)

	body2, _ := os.ReadFile(path)
	var doc2 map[string]any
	require.NoError(t, json.Unmarshal(body2, &doc2))
	mcps := doc2["mcp"].(map[string]any)
	require.Equal(t, false, mcps["engram"].(map[string]any)["command"])
	require.Equal(t, false, mcps["mem0"].(map[string]any)["command"])

	require.Equal(t, []any{"/domain"}, mcps["domain"].(map[string]any)["command"])
	require.Equal(t, []any{"/fs"}, mcps["filesystem"].(map[string]any)["command"])
}

func TestDisable_NotAMemoryProvider_NotModified(t *testing.T) {



	entry := map[string]any{"command": []any{"/fs"}}
	path := writeConfigWithEntry(t, "opencode", "filesystem", entry)
	err := Disable("opencode", path, []string{"filesystem"})
	require.NoError(t, err)
	body, _ := os.ReadFile(path)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	mcps := doc["mcp"].(map[string]any)
	require.Equal(t, false, mcps["filesystem"].(map[string]any)["command"],
		"Disable es dumb — caller debe filtrar por IsMemory")
}

func TestDisable_ProviderNotInConfig_NoOp(t *testing.T) {

	entry := map[string]any{"command": []any{"/domain"}}
	path := writeConfigWithEntry(t, "opencode", "domain", entry)
	err := Disable("opencode", path, []string{"engram"})
	require.NoError(t, err)
	body, _ := os.ReadFile(path)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	mcps := doc["mcp"].(map[string]any)
	require.Equal(t, []any{"/domain"}, mcps["domain"].(map[string]any)["command"],
		"domain no se debe tocar")
}

func TestDisable_BackupBeforeAnyChange(t *testing.T) {



	entry := map[string]any{"command": []any{"/engram"}}
	path := writeConfigWithEntry(t, "opencode", "engram", entry)

	dir := filepath.Dir(path)

	before, _ := os.ReadDir(dir)
	var beforeNames []string
	for _, e := range before {
		beforeNames = append(beforeNames, e.Name())
	}

	err := Disable("opencode", path, []string{"engram"})
	require.NoError(t, err)

	after, _ := os.ReadDir(dir)
	var afterNames []string
	for _, e := range after {
		afterNames = append(afterNames, e.Name())
	}

	require.Greater(t, len(afterNames), len(beforeNames),
		"debe aparecer al menos 1 archivo nuevo (backup). before=%v after=%v", beforeNames, afterNames)
}

func TestReactivate_RestoresFromBackup(t *testing.T) {
	entry := map[string]any{"command": []any{"/engram"}, "enabled": true}
	path := writeConfigWithEntry(t, "opencode", "engram", entry)


	require.NoError(t, Disable("opencode", path, []string{"engram"}))

	disabled, _ := IsAlreadyDisabled("opencode", path, "engram")
	require.True(t, disabled, "debe estar disabled post-Disable")


	err := Reactivate("opencode", path)
	require.NoError(t, err)
	disabled, _ = IsAlreadyDisabled("opencode", path, "engram")
	require.False(t, disabled, "debe estar restaurado post-Reactivate")
}

func TestReactivate_NoBackup_ReturnsError(t *testing.T) {
	entry := map[string]any{"command": []any{"/engram"}}
	path := writeConfigWithEntry(t, "opencode", "engram", entry)

	err := Reactivate("opencode", path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no backup")
}

func TestReactivate_MultipleBackups_PicksLatest(t *testing.T) {




	entry := map[string]any{"command": []any{"/engram"}}
	path := writeConfigWithEntry(t, "opencode", "engram", entry)


	require.NoError(t, Disable("opencode", path, []string{"engram"}))
	time.Sleep(1100 * time.Millisecond) // ensure different timestamp (segundos)

	require.NoError(t, Disable("opencode", path, []string{"engram"}))



	require.NoError(t, Reactivate("opencode", path))
	body, _ := os.ReadFile(path)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	mcps := doc["mcp"].(map[string]any)
	require.Equal(t, false, mcps["engram"].(map[string]any)["command"],
		"1ra reactivate debe restaurar el estado disabled del último disable")



	require.NoError(t, Reactivate("opencode", path))
	body2, _ := os.ReadFile(path)
	var doc2 map[string]any
	require.NoError(t, json.Unmarshal(body2, &doc2))
	mcps2 := doc2["mcp"].(map[string]any)
	require.Equal(t, []any{"/engram"}, mcps2["engram"].(map[string]any)["command"],
		"2da reactivate debe restaurar el estado original")
}

func TestDisable_Idempotent_SecondCallNoOp(t *testing.T) {
	entry := map[string]any{"command": []any{"/engram"}}
	path := writeConfigWithEntry(t, "opencode", "engram", entry)


	require.NoError(t, Disable("opencode", path, []string{"engram"}))
	body1, _ := os.ReadFile(path)


	require.NoError(t, Disable("opencode", path, []string{"engram"}))
	body2, _ := os.ReadFile(path)


	var d1, d2 map[string]any
	require.NoError(t, json.Unmarshal(body1, &d1))
	require.NoError(t, json.Unmarshal(body2, &d2))
	mcps1 := d1["mcp"].(map[string]any)
	mcps2 := d2["mcp"].(map[string]any)
	require.Equal(t, false, mcps1["engram"].(map[string]any)["command"])
	require.Equal(t, false, mcps2["engram"].(map[string]any)["command"])
}
