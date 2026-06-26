package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// opencode.json sin contenedor "mcp" → se crea y se agrega entry
func TestConfigureClient_OpencodeNoMcpContainer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	cfgDir := filepath.Join(home, ".config", "opencode")
	writeFakeClientConfig(t, cfgDir, "opencode.json", `{"$schema":"https://opencode.ai/config.json"}`)

	c := Client{Name: "opencode", MCPPath: filepath.Join(cfgDir, "opencode.json")}
	if err := configureClient(c, "http://vps.example", "domk_test", "20260620T000000Z"); err != nil {
		t.Fatalf("configureClient: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(cfgDir, "opencode.json"))
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	mcp, ok := m["mcp"].(map[string]any)
	if !ok {
		t.Fatal("contenedor mcp no fue creado")
	}
	dm := mcp["domain-mcp"].(map[string]any)
	if dm["type"] != "remote" {
		t.Errorf("type = %v, want 'remote'", dm["type"])
	}
	if m["$schema"] != "https://opencode.ai/config.json" {
		t.Error("$schema del usuario fue perdido")
	}
}

// otros servers del usuario (atlassian, slack) NO se tocan
func TestConfigureClient_ClaudeCode_PreservesAllOthers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	claudeDir := filepath.Join(home, ".claude")
	original := `{"mcpServers":{"atlassian":{"url":"https://x"},"slack":{"command":"node"},"domain":{"url":"OLD"}}}`
	writeFakeClientConfig(t, claudeDir, "mcp_servers.json", original)

	c := Client{Name: "claude-code", MCPPath: filepath.Join(claudeDir, "mcp_servers.json")}
	if err := configureClient(c, "http://vps.example", "domk_test", "20260620T000000Z"); err != nil {
		t.Fatalf("configureClient: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(claudeDir, "mcp_servers.json"))
	var m map[string]any
	json.Unmarshal(raw, &m)
	servers := m["mcpServers"].(map[string]any)

	for _, name := range []string{"atlassian", "slack"} {
		if _, ok := servers[name]; !ok {
			t.Errorf("server del usuario '%s' fue pisado", name)
		}
	}
	if _, ok := servers["domain"]; ok {
		t.Error("entry legacy 'domain' no fue migrada/eliminada")
	}
	dm := servers["domain-mcp"].(map[string]any)
	if dm["url"] != "http://vps.example/mcp" {
		t.Errorf("domain-mcp url = %v", dm["url"])
	}
}

// backup se crea con timestamp correcto y NO pisa al actual
func TestConfigureClient_BackupCreated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	claudeDir := filepath.Join(home, ".claude")
	writeFakeClientConfig(t, claudeDir, "mcp_servers.json", `{"original":true}`)

	c := Client{Name: "claude-code", MCPPath: filepath.Join(claudeDir, "mcp_servers.json")}
	if err := configureClient(c, "http://vps.example", "domk_test", "20260620T000000Z"); err != nil {
		t.Fatalf("configureClient: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join(claudeDir, "mcp_servers.json.backup-*"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 backup, got %d: %v", len(matches), matches)
	}
	if !strings.Contains(matches[0], "20260620T000000Z") {
		t.Errorf("backup name = %q, want contiene timestamp", matches[0])
	}
}
