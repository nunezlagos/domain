package autodetect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetect_StateNone(t *testing.T) {
	dir := t.TempDir()
	st, err := Detect(dir)
	require.NoError(t, err)
	require.Equal(t, StateNone, st)
}

func TestDetect_StateClaudeMDOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# test"), 0o644))
	st, err := Detect(dir)
	require.NoError(t, err)
	require.Equal(t, StateClaudeMDOnly, st)
}

func TestDetect_StateMCPJSONOnly(t *testing.T) {
	dir := t.TempDir()
	mcp := map[string]any{"mcpServers": map[string]any{"opsx": map[string]any{"command": "/bin/opsx"}}}
	data, _ := json.MarshalIndent(mcp, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), data, 0o644))
	st, err := Detect(dir)
	require.NoError(t, err)
	require.Equal(t, StateMCPJSONOnly, st)
}

func TestDetect_StateOpenCodeConfigOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(`{"mcp":{}}`), 0o644))
	st, err := Detect(dir)
	require.NoError(t, err)
	require.Equal(t, StateOpenCodeConfigOnly, st)
}

func TestDetect_InvalidPath(t *testing.T) {
	_, err := Detect("/nonexistent/path")
	require.Error(t, err)
}

func TestDetect_PathIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("content"), 0o644))
	_, err := Detect(file)
	require.Error(t, err)
}

func TestApply_SymlinkOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project"), 0o644))

	actions, err := Apply(dir)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, ActionSymlink, actions[0].Type)
	require.Equal(t, "AGENTS.md", actions[0].Path)
	require.Equal(t, "CLAUDE.md", actions[0].Target)

	target, err := os.Readlink(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	require.Equal(t, "CLAUDE.md", target)

	m, err := ReadManifest(filepath.Join(dir, ".domain", "install-manifest.json"))
	require.NoError(t, err)
	require.Len(t, m.Actions, 1)
}

func TestApply_JSONUpsertMCP(t *testing.T) {
	dir := t.TempDir()
	mcp := map[string]any{"mcpServers": map[string]any{"opsx": map[string]any{"command": "/bin/opsx"}}}
	data, _ := json.MarshalIndent(mcp, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), data, 0o644))

	actions, err := Apply(dir)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, ActionJSONUpsert, actions[0].Type)
	require.Equal(t, ".mcp.json", actions[0].Path)
	require.Equal(t, "mcpServers.domain", actions[0].Key)

	var doc map[string]any
	raw, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	require.NoError(t, json.Unmarshal(raw, &doc))
	servers := doc["mcpServers"].(map[string]any)
	require.Contains(t, servers, "domain")
	require.Contains(t, servers, "opsx")
}

func TestApply_Idempotent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project"), 0o644))
	mcp := map[string]any{"mcpServers": map[string]any{"opsx": map[string]any{"command": "/bin/opsx"}}}
	data, _ := json.MarshalIndent(mcp, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), data, 0o644))

	actions1, err := Apply(dir)
	require.NoError(t, err)
	// Both symlink and MCP upsert needed
	require.GreaterOrEqual(t, len(actions1), 1)

	actions2, err := Apply(dir)
	require.NoError(t, err)
	require.Len(t, actions2, 0, "second apply should be no-op")
}

func TestApply_NoChangesNeededEmptyDir(t *testing.T) {
	dir := t.TempDir()
	actions, err := Apply(dir)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, ActionOpenCodeGen, actions[0].Type)

	_, err = os.Stat(filepath.Join(dir, "opencode.json"))
	require.NoError(t, err)
}

func TestApply_DryRun(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project"), 0o644))

	actions, err := ApplyDryRun(dir)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, ActionSymlink, actions[0].Type)

	_, err = os.Stat(filepath.Join(dir, "AGENTS.md"))
	require.True(t, os.IsNotExist(err))
}

func TestApply_FullProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project"), 0o644))
	mcp := map[string]any{"mcpServers": map[string]any{"opsx": map[string]any{"command": "/bin/opsx"}}}
	data, _ := json.MarshalIndent(mcp, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), data, 0o644))

	actions, err := Apply(dir)
	require.NoError(t, err)
	// Symlink + MCP upsert
	require.GreaterOrEqual(t, len(actions), 2)

	target, err := os.Readlink(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	require.Equal(t, "CLAUDE.md", target)

	var doc map[string]any
	raw, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	require.NoError(t, json.Unmarshal(raw, &doc))
	servers := doc["mcpServers"].(map[string]any)
	require.Contains(t, servers, "domain")

	m, err := ReadManifest(filepath.Join(dir, ".domain", "install-manifest.json"))
	require.NoError(t, err)
	require.Len(t, m.Actions, len(actions))
}

func TestManifest_ReadWrite(t *testing.T) {
	dir := t.TempDir()
	m := Manifest{
		Version:       1,
		DomainVersion: "0.1.0",
		Actions: []Action{
			{Type: ActionSymlink, Path: "AGENTS.md", Target: "CLAUDE.md"},
		},
	}
	path := filepath.Join(dir, "manifest.json")
	require.NoError(t, WriteManifest(path, &m))

	got, err := ReadManifest(path)
	require.NoError(t, err)
	require.Equal(t, m.Version, got.Version)
	require.Equal(t, m.DomainVersion, got.DomainVersion)
	require.Len(t, got.Actions, 1)
	require.Equal(t, ActionSymlink, got.Actions[0].Type)
}

func TestManifest_ReadNonexistent(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadManifest(filepath.Join(dir, "nope.json"))
	require.Error(t, err)
}

func TestManifest_Append(t *testing.T) {
	dir := t.TempDir()
	domainDir := filepath.Join(dir, ".domain")
	require.NoError(t, os.MkdirAll(domainDir, 0o755))
	path := filepath.Join(domainDir, "install-manifest.json")

	m := Manifest{Version: 1, DomainVersion: "0.1.0"}
	require.NoError(t, WriteManifest(path, &m))
	require.NoError(t, AppendToManifest(path, Action{Type: ActionSymlink, Path: "AGENTS.md", Target: "CLAUDE.md"}))

	got, err := ReadManifest(path)
	require.NoError(t, err)
	require.Len(t, got.Actions, 1)
}

func TestManifest_AppendNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".domain", "fresh-manifest.json")
	require.NoError(t, AppendToManifest(path, Action{Type: ActionSymlink, Path: "AGENTS.md", Target: "CLAUDE.md"}))

	got, err := ReadManifest(path)
	require.NoError(t, err)
	require.Len(t, got.Actions, 1)
}

func TestApply_MCPJSONWithExistingDomain(t *testing.T) {
	dir := t.TempDir()
	mcp := map[string]any{
		"mcpServers": map[string]any{
			"domain": map[string]any{"command": "/usr/local/bin/domain-mcp"},
		},
	}
	data, _ := json.MarshalIndent(mcp, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), data, 0o644))

	actions, err := Apply(dir)
	require.NoError(t, err)
	require.Len(t, actions, 0, "if domain already in .mcp.json, no action needed")
}

func TestApply_SymlinkAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# test"), 0o644))
	require.NoError(t, os.Symlink("CLAUDE.md", filepath.Join(dir, "AGENTS.md")))

	actions, err := Apply(dir)
	require.NoError(t, err)
	require.Len(t, actions, 0, "if symlink already exists, no action needed")
}
