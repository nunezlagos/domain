package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// confirm: "(Y/n)" implica default-yes → Enter (vacío) y 'y'/'yes' = true;
// 'n'/'no' o cualquier otra cosa = false.
func TestConfirm_DefaultYesOnEmpty(t *testing.T) {
	cases := map[string]bool{
		"\n":    true, // Enter solo → default yes
		"y\n":   true,
		"yes\n": true,
		"n\n":   false,
		"no\n":  false,
		"x\n":   false,
	}
	for in, want := range cases {
		got := confirm(bufio.NewReader(strings.NewReader(in)), "? ")
		if got != want {
			t.Errorf("confirm(%q) = %v, want %v", in, got, want)
		}
	}
}

// opencode.json sin contenedor "mcp" → se crea y se agrega entry
func TestConfigureClient_OpencodeNoMcpContainer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	cfgDir := filepath.Join(home, ".config", "opencode")
	writeFakeClientConfig(t, cfgDir, "opencode.json", `{"$schema":"https://opencode.ai/config.json"}`)

	c := Client{Name: "opencode", MCPPath: filepath.Join(cfgDir, "opencode.json")}
	if _, err := configureClient(c, "http://vps.example", "domk_test", "20260620T000000Z"); err != nil {
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

	home2 := filepath.Join(home, "h")
	original := `{"mcpServers":{"atlassian":{"url":"https://x"},"slack":{"command":"node"},"domain":{"url":"OLD"}}}`
	writeFakeClientConfig(t, home2, ".claude.json", original)

	c := Client{Name: "claude-code", MCPPath: filepath.Join(home2, ".claude.json")}
	if _, err := configureClient(c, "http://vps.example", "domk_test", "20260620T000000Z"); err != nil {
		t.Fatalf("configureClient: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(home2, ".claude.json"))
	var m map[string]any
	json.Unmarshal(raw, &m)
	servers := m["mcpServers"].(map[string]any)

	for _, name := range []string{"atlassian", "slack"} {
		if _, ok := servers[name]; !ok {
			t.Errorf("server del usuario '%s' fue pisado", name)
		}
	}
	if _, ok := servers["domain"]; ok {
		t.Error("entry legacy 'domain' (remota) no fue migrada/eliminada")
	}
	dm := servers["domain-mcp"].(map[string]any)
	if dm["url"] != "http://vps.example/mcp" {
		t.Errorf("domain-mcp url = %v", dm["url"])
	}
	if dm["type"] != "http" {
		t.Errorf("domain-mcp type = %v, want 'http' (Claude Code remoto)", dm["type"])
	}
}

// Dedup: si ya existe un 'domain' LOCAL (instalador del server, con command),
// NO se agrega un 'domain-mcp' remoto ni se pisa el local.
func TestConfigureClient_ClaudeCode_SkipsWhenLocalDomainExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	original := `{"mcpServers":{"domain":{"command":"/usr/local/bin/domain-mcp","args":[]}}}`
	writeFakeClientConfig(t, home, ".claude.json", original)

	c := Client{Name: "claude-code", MCPPath: filepath.Join(home, ".claude.json")}
	res, err := configureClient(c, "http://vps.example", "domk_test", "20260620T000000Z")
	if err != nil {
		t.Fatalf("configureClient: %v", err)
	}
	if !res.Skipped {
		t.Fatal("esperaba Skipped=true por dedup local↔remoto")
	}

	raw, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	var m map[string]any
	json.Unmarshal(raw, &m)
	servers := m["mcpServers"].(map[string]any)
	if _, ok := servers["domain-mcp"]; ok {
		t.Error("se creó 'domain-mcp' remoto pese a existir 'domain' local")
	}
	dm, ok := servers["domain"].(map[string]any)
	if !ok || dm["command"] != "/usr/local/bin/domain-mcp" {
		t.Error("'domain' local fue alterado")
	}
	// idempotente: no debe dejar backup si no tocó nada
	matches, _ := filepath.Glob(filepath.Join(home, ".claude.json.backup-*"))
	if len(matches) != 0 {
		t.Errorf("skip creó backup innecesario: %v", matches)
	}
}

// Claude Desktop: stdio-only → se omite con razón, no escribe config inútil.
func TestConfigureClient_ClaudeDesktop_SkippedRemote(t *testing.T) {
	res, err := configureClient(
		Client{Name: "claude-desktop", MCPPath: filepath.Join(t.TempDir(), "claude_desktop_config.json")},
		"http://vps.example", "domk_test", "20260620T000000Z")
	if err != nil {
		t.Fatalf("configureClient: %v", err)
	}
	if !res.Skipped || res.Reason == "" {
		t.Errorf("esperaba skip con razón para claude-desktop, got %+v", res)
	}
}

// backup se crea con timestamp correcto y NO pisa al actual
func TestConfigureClient_BackupCreated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	writeFakeClientConfig(t, home, ".claude.json", `{"original":true}`)

	c := Client{Name: "claude-code", MCPPath: filepath.Join(home, ".claude.json")}
	if _, err := configureClient(c, "http://vps.example", "domk_test", "20260620T000000Z"); err != nil {
		t.Fatalf("configureClient: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join(home, ".claude.json.backup-*"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 backup, got %d: %v", len(matches), matches)
	}
	if !strings.Contains(matches[0], "20260620T000000Z") {
		t.Errorf("backup name = %q, want contiene timestamp", matches[0])
	}
}
