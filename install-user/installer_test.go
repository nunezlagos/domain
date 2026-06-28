package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFakeClientConfig(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildInstallPlan_NoClients_NoAutoInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())

	plan, err := BuildInstallPlan(InstallOptions{AutoInstallOpencode: false})
	if err != nil {
		t.Fatalf("BuildInstallPlan: %v", err)
	}
	if !plan.NeedsOpencode {
		t.Error("NeedsOpencode = false, want true (no se detectó ningún cliente)")
	}
	if len(plan.Targets) != 0 {
		t.Errorf("Targets = %v, want empty", plan.Targets)
	}
}

func TestBuildInstallPlan_NoClients_WithAutoInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())

	plan, err := BuildInstallPlan(InstallOptions{AutoInstallOpencode: true})
	if err != nil {
		t.Fatalf("BuildInstallPlan: %v", err)
	}
	if !plan.NeedsOpencode {
		t.Error("NeedsOpencode = false, want true")
	}
	if plan.OpencodeCmd.Primary == nil && plan.OpencodeCmd.Fallback == nil {
		t.Error("OpencodeCmd vacía, debería tener al menos Fallback npm")
	}
}

func TestBuildInstallPlan_OneClient(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	writeFakeClientConfig(t, filepath.Join(home, ".config", "opencode"), "opencode.json", `{}`)

	plan, err := BuildInstallPlan(InstallOptions{AutoInstallOpencode: false})
	if err != nil {
		t.Fatalf("BuildInstallPlan: %v", err)
	}
	if plan.NeedsOpencode {
		t.Error("NeedsOpencode = true, want false (opencode detectado)")
	}
	if len(plan.Targets) != 1 || plan.Targets[0].Name != "opencode" {
		t.Errorf("Targets = %+v, want [opencode]", plan.Targets)
	}
}

func TestBuildInstallPlan_TargetFilter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	writeFakeClientConfig(t, filepath.Join(home, ".config", "opencode"), "opencode.json", `{}`)
	writeFakeClientConfig(t, home, ".claude.json", `{}`)

	plan, err := BuildInstallPlan(InstallOptions{Target: "opencode"})
	if err != nil {
		t.Fatalf("BuildInstallPlan: %v", err)
	}
	if len(plan.Targets) != 1 || plan.Targets[0].Name != "opencode" {
		t.Errorf("Targets = %+v, want solo [opencode]", plan.Targets)
	}
}

func TestBuildInstallPlan_InvalidTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	_, err := BuildInstallPlan(InstallOptions{Target: "no-existe"})
	if err == nil {
		t.Fatal("expected error para target inválido")
	}
	if !strings.Contains(err.Error(), "no-existe") {
		t.Errorf("err = %q, want contiene 'no-existe'", err.Error())
	}
}

// Ping al VPS — healthz retorna 200 → OK
func TestPingVPS_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("path = %q, want /healthz", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pingVPS(ctx, srv.URL); err != nil {
		t.Errorf("pingVPS: %v", err)
	}
}

// Ping al VPS — server cerrado → error
func TestPingVPS_ServerDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := pingVPS(ctx, "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ping failed") {
		t.Errorf("err = %q, want contiene 'ping failed'", err.Error())
	}
}

// Ping al VPS — status 500 → error
func TestPingVPS_500IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := pingVPS(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected error para status 500")
	}
}

// Apply plan: crea opencode.json con entry domain-mcp
func TestApply_OpencodeRemoteEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	cfgDir := filepath.Join(home, ".config", "opencode")
	writeFakeClientConfig(t, cfgDir, "opencode.json", `{"mcp":{"context7":{"type":"remote","url":"https://mcp.context7.com/mcp"}}}`)

	plan := InstallPlan{
		Targets: []Client{{Name: "opencode", MCPPath: filepath.Join(cfgDir, "opencode.json")}},
	}
	if _, err := Apply(plan, "http://test.local", "domk_live_test"); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(cfgDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	mcp := m["mcp"].(map[string]any)
	if _, ok := mcp["context7"]; !ok {
		t.Error("context7 (del usuario) fue pisado")
	}
	dm, ok := mcp["domain-mcp"].(map[string]any)
	if !ok {
		t.Fatal("domain-mcp no se creó")
	}
	if dm["url"] != "http://test.local/mcp" {
		t.Errorf("url = %v, want 'http://test.local/mcp'", dm["url"])
	}
	if dm["type"] != "remote" {
		t.Errorf("type = %v, want 'remote'", dm["type"])
	}
}

// Apply: opencode.json preexistente sin `type: remote` (schema viejo) → agregar
func TestApply_OpencodeSchemaUpgrade(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	cfgDir := filepath.Join(home, ".config", "opencode")
	writeFakeClientConfig(t, cfgDir, "opencode.json", `{"mcp":{"old":{"url":"http://old.example"}}}`)

	plan := InstallPlan{
		Targets: []Client{{Name: "opencode", MCPPath: filepath.Join(cfgDir, "opencode.json")}},
	}
	if _, err := Apply(plan, "http://test.local", "domk_live_test"); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(cfgDir, "opencode.json"))
	var m map[string]any
	json.Unmarshal(raw, &m)
	dm := m["mcp"].(map[string]any)["domain-mcp"].(map[string]any)
	if dm["type"] != "remote" {
		t.Errorf("schema upgrade: type = %v, want 'remote'", dm["type"])
	}
	if dm["enabled"] != true {
		t.Errorf("schema upgrade: enabled = %v, want true", dm["enabled"])
	}
	oldEntry := m["mcp"].(map[string]any)["old"].(map[string]any)
	if oldEntry["url"] != "http://old.example" {
		t.Error("entry legacy del usuario fue pisada")
	}
}
