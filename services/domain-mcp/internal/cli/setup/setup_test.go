package setup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClaudeDesktopConfigPath_OSDependent(t *testing.T) {
	path, err := ClaudeDesktopConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	switch runtime.GOOS {
	case "darwin":
		if !contains(path, "Library/Application Support/Claude") {
			t.Fatalf("darwin path: %s", path)
		}
	case "linux":
		if !contains(path, ".config/Claude") {
			t.Fatalf("linux path: %s", path)
		}
	}
}

func TestSetupClaudeDesktop_CreaConfigNuevo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))

	path, err := SetupClaudeDesktop("/usr/local/bin/domain-mcp", "sk_test", "http://localhost:8000")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg ClaudeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	srv, ok := cfg.MCPServers["domain"]
	if !ok {
		t.Fatal("domain server missing")
	}
	if srv.Command != "/usr/local/bin/domain-mcp" {
		t.Fatalf("command: %s", srv.Command)
	}
	if srv.Env["DOMAIN_API_KEY"] != "sk_test" {
		t.Fatalf("api_key not propagated: %v", srv.Env)
	}
}

func TestSetupClaudeDesktop_AlreadyConfiguredError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)


	cfgPath, err := ClaudeDesktopConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0o755)
	pre := ClaudeConfig{
		MCPServers: map[string]MCPServerConfig{
			"domain": {Command: "/old/path"},
		},
	}
	body, _ := json.MarshalIndent(pre, "", "  ")
	_ = os.WriteFile(cfgPath, body, 0o600)

	_, err = SetupClaudeDesktop("/new/path", "", "")
	if !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatalf("expected ErrAlreadyConfigured, got %v", err)
	}
}

func TestSetupClaudeDesktop_PreservaConfigsExistentes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfgPath, _ := ClaudeDesktopConfigPath()
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0o755)
	pre := ClaudeConfig{
		MCPServers: map[string]MCPServerConfig{
			"github": {Command: "/usr/bin/gh-mcp"},
		},
	}
	body, _ := json.MarshalIndent(pre, "", "  ")
	_ = os.WriteFile(cfgPath, body, 0o600)

	_, err := SetupClaudeDesktop("/usr/local/bin/domain-mcp", "", "")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	data, _ := os.ReadFile(cfgPath)
	var cfg ClaudeConfig
	_ = json.Unmarshal(data, &cfg)
	if _, ok := cfg.MCPServers["github"]; !ok {
		t.Fatal("server github eliminado por error")
	}
	if _, ok := cfg.MCPServers["domain"]; !ok {
		t.Fatal("server domain no agregado")
	}
}

func TestCreateAIDirectives_NoSobreescribe(t *testing.T) {
	tmp := t.TempDir()
	path, err := CreateAIDirectives(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("manual content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := CreateAIDirectives(tmp); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "manual content" {
		t.Fatal("sobrescribió contenido manual")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
