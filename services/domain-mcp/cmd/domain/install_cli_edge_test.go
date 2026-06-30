














package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepairOpencodeEmptyCommand_NoFile(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)


	require.False(t, repairOpencodeEmptyCommand())
}

func TestRepairOpencodeEmptyCommand_NoMcpKey(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)

	require.NoError(t, os.WriteFile("opencode.json", []byte(`{"mcp":{}}`), 0o600))


	require.False(t, repairOpencodeEmptyCommand())
}

func TestRepairOpencodeEmptyCommand_ValidCommand_NoRepair(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)


	binPath := dir + "/domain-mcp"
	require.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755))
	content := `{"mcp":{"domain":{"command":["` + binPath + `"],"enabled":true,"type":"local"}}}`
	require.NoError(t, os.WriteFile("opencode.json", []byte(content), 0o600))


	require.False(t, repairOpencodeEmptyCommand())


	data, _ := os.ReadFile("opencode.json")
	require.Equal(t, content, string(data))
}

func TestRepairOpencodeEmptyCommand_MissingBinary_Repairs(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)



	content := `{"mcp":{"domain":{"command":["/home/x/.local/bin/domain-mcp"],"enabled":true,"type":"local"}}}`
	require.NoError(t, os.WriteFile("opencode.json", []byte(content), 0o600))

	require.True(t, repairOpencodeEmptyCommand())

	data, _ := os.ReadFile("opencode.json")
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	mcp := doc["mcp"].(map[string]any)
	_, exists := mcp["domain"]
	require.False(t, exists, "entry con binario inexistente debe borrarse")
}

func TestRepairOpencodeEmptyCommand_EmptyString_Repairs(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)


	content := `{"mcp":{"domain":{"command":[""],"enabled":true,"type":"local"},"other":{"x":1}}}`
	require.NoError(t, os.WriteFile("opencode.json", []byte(content), 0o600))


	require.True(t, repairOpencodeEmptyCommand())


	data, _ := os.ReadFile("opencode.json")
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	mcp := doc["mcp"].(map[string]any)
	_, exists := mcp["domain"]
	require.False(t, exists, "domain entry must be removed when command is empty")

	_, otherExists := mcp["other"]
	require.True(t, otherExists, "other MCP entries must NOT be removed")
}

func TestRepairOpencodeEmptyCommand_MissingField_Repairs(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)


	content := `{"mcp":{"domain":{"command":[],"enabled":true}}}`
	require.NoError(t, os.WriteFile("opencode.json", []byte(content), 0o600))

	require.True(t, repairOpencodeEmptyCommand())
}

func TestRepairOpencodeEmptyCommand_PreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)


	content := `{
		"$schema": "https://opencode.ai/config.json",
		"enabled": true,
		"mcp": {
			"context7": {"type": "remote", "url": "https://mcp.context7.com/mcp", "enabled": true},
			"domain": {"command": [""], "enabled": true, "type": "local"},
			"fetch": {"type": "local", "command": ["uvx", "mcp-server-fetch"], "enabled": true}
		}
	}`
	require.NoError(t, os.WriteFile("opencode.json", []byte(content), 0o600))

	require.True(t, repairOpencodeEmptyCommand())


	data, _ := os.ReadFile("opencode.json")
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))

	require.Equal(t, "https://opencode.ai/config.json", doc["$schema"])
	require.Equal(t, true, doc["enabled"])

	mcp := doc["mcp"].(map[string]any)
	_, domainExists := mcp["domain"]
	require.False(t, domainExists)
	_, ctxExists := mcp["context7"]
	require.True(t, ctxExists, "context7 must remain")
	_, fetchExists := mcp["fetch"]
	require.True(t, fetchExists, "fetch must remain")
}

func TestRepairOpencodeEmptyCommand_InvalidJSON_NoCrash(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)

	require.NoError(t, os.WriteFile("opencode.json", []byte("not valid json{{{"), 0o600))


	require.False(t, repairOpencodeEmptyCommand())
}

// TestEnsureLocalEnvFile_CommitsWithStatusFile preserva el test
// pre-existente — no toco.
var _ = filepath.Join // para evitar unused import si se eliminan tests
