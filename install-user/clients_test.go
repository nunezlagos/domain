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

// issue-65.1 G1: configureContinue debe PRESERVAR los MCP servers ajenos del
// usuario en experimental.modelContextProtocolServers, no reemplazar el array.

func TestConfigureContinue_PreservaOtrosServers(t *testing.T) {
	dir := t.TempDir()
	original := `{"experimental":{"modelContextProtocolServers":[` +
		`{"transport":{"type":"http","url":"https://otro.example/mcp"}},` +
		`{"transport":{"type":"stdio","command":"foo"}}` +
		`]}}`
	writeFakeClientConfig(t, dir, "config.json", original)
	path := filepath.Join(dir, "config.json")

	if err := configureContinue(path, "http://vps.example", "domk_test", "20260620T000000Z"); err != nil {
		t.Fatalf("configureContinue: %v", err)
	}

	raw, _ := os.ReadFile(path)
	var m map[string]any
	json.Unmarshal(raw, &m)
	exp := m["experimental"].(map[string]any)
	servers := exp["modelContextProtocolServers"].([]any)

	// Debe haber 3: los 2 ajenos + domain.
	if len(servers) != 3 {
		t.Fatalf("esperaba 3 servers (2 ajenos + domain), hay %d", len(servers))
	}
	var hasOtro, hasFoo, hasDomain bool
	for _, s := range servers {
		sm := s.(map[string]any)
		tr, _ := sm["transport"].(map[string]any)
		url, _ := tr["url"].(string)
		cmd, _ := tr["command"].(string)
		if url == "https://otro.example/mcp" {
			hasOtro = true
		}
		if cmd == "foo" {
			hasFoo = true
		}
		if url == "http://vps.example/mcp" {
			hasDomain = true
		}
	}
	if !hasOtro || !hasFoo {
		t.Errorf("servers ajenos del usuario fueron pisados (otro=%v foo=%v)", hasOtro, hasFoo)
	}
	if !hasDomain {
		t.Error("el server de domain no fue agregado")
	}
}

func TestConfigureContinue_NoDuplicaDomain(t *testing.T) {
	dir := t.TempDir()
	original := `{"experimental":{"modelContextProtocolServers":[` +
		`{"transport":{"type":"http","url":"http://vps.example/mcp","headers":{"Authorization":"Bearer viejo"}}}` +
		`]}}`
	writeFakeClientConfig(t, dir, "config.json", original)
	path := filepath.Join(dir, "config.json")

	if err := configureContinue(path, "http://vps.example", "domk_nueva", "20260620T000000Z"); err != nil {
		t.Fatalf("configureContinue: %v", err)
	}
	raw, _ := os.ReadFile(path)
	var m map[string]any
	json.Unmarshal(raw, &m)
	servers := m["experimental"].(map[string]any)["modelContextProtocolServers"].([]any)
	if len(servers) != 1 {
		t.Fatalf("re-configurar duplicó la entrada domain: hay %d, esperaba 1", len(servers))
	}
}

// issue-65.1 G1: uninstall de continue debe crear backup antes de escribir.
func TestUninstallContinue_BackupAntesDeEscribir(t *testing.T) {
	dir := t.TempDir()
	original := `{"experimental":{"modelContextProtocolServers":[` +
		`{"transport":{"type":"http","url":"http://vps.example/mcp"}}` +
		`]}}`
	writeFakeClientConfig(t, dir, "config.json", original)
	path := filepath.Join(dir, "config.json")

	removed, err := uninstallClient(Client{Name: "continue", MCPPath: path})
	if err != nil {
		t.Fatalf("uninstallClient: %v", err)
	}
	if !removed {
		t.Fatal("esperaba removed=true (había 1 server domain en continue)")
	}
	// Debe existir al menos un backup del config.json.
	entries, _ := filepath.Glob(path + ".backup-*")
	if len(entries) == 0 {
		t.Error("no se creó backup del config.json antes del write en uninstall")
	}
}

// issue-65.1 (hallazgo panel): si la entrada domain existía con OTRA url (VPS
// migró), el merge debe ACTUALIZARLA in-place, no dejar una stale duplicada.
func TestConfigureContinue_VpsMigrado_NoDuplica(t *testing.T) {
	dir := t.TempDir()
	original := `{"experimental":{"modelContextProtocolServers":[` +
		`{"transport":{"type":"http","url":"http://viejo-vps.example/mcp","headers":{"Authorization":"Bearer domk_vieja"}}},` +
		`{"transport":{"type":"http","url":"https://ajeno.example/api"}}` +
		`]}}`
	writeFakeClientConfig(t, dir, "config.json", original)
	path := filepath.Join(dir, "config.json")

	if err := configureContinue(path, "http://nuevo-vps.example", "domk_nueva", "20260620T000000Z"); err != nil {
		t.Fatalf("configureContinue: %v", err)
	}
	raw, _ := os.ReadFile(path)
	var m map[string]any
	json.Unmarshal(raw, &m)
	servers := m["experimental"].(map[string]any)["modelContextProtocolServers"].([]any)

	// Debe haber 2: la entrada domain (actualizada al nuevo VPS) + el server ajeno.
	// NO 3 (no debe quedar la entrada vieja del VPS migrado).
	if len(servers) != 2 {
		t.Fatalf("VPS migrado dejó entrada domain stale: hay %d servers, esperaba 2", len(servers))
	}
	var domainURL, headerKey string
	var hasAjeno bool
	for _, s := range servers {
		sm := s.(map[string]any)
		tr := sm["transport"].(map[string]any)
		url, _ := tr["url"].(string)
		if strings.HasSuffix(url, "/mcp") {
			domainURL = url
			if h, ok := tr["headers"].(map[string]any); ok {
				headerKey, _ = h["Authorization"].(string)
			}
		}
		if url == "https://ajeno.example/api" {
			hasAjeno = true
		}
	}
	if domainURL != "http://nuevo-vps.example/mcp" {
		t.Errorf("la entrada domain no se actualizó al nuevo VPS: url=%q", domainURL)
	}
	// Hallazgo del juez A: el header Authorization debe refrescarse a la key nueva.
	if headerKey != "Bearer domk_nueva" {
		t.Errorf("el header Authorization no se refrescó: %q", headerKey)
	}
	if !hasAjeno {
		t.Error("el server ajeno del usuario fue pisado")
	}
}
