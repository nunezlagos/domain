package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// "Bearer domk_live_XYZ" → "domk_live_XYZ"
func TestExtractAPIKeyFromBearer_Valid(t *testing.T) {
	got := extractAPIKeyFromBearer("Bearer domk_live_18vqUCiHEqI53kuUZYx4Y76agbZg_rYVs6dN5l0XA3I")
	if got != "domk_live_18vqUCiHEqI53kuUZYx4Y76agbZg_rYVs6dN5l0XA3I" {
		t.Errorf("got %q", got)
	}
}

// "Bearer domk_test_XYZ" → "domk_test_XYZ"
func TestExtractAPIKeyFromBearer_TestKey(t *testing.T) {
	in := "Bearer domk_test_aaa111bbb222ccc333ddd444eee555"
	if got := extractAPIKeyFromBearer(in); got != "domk_test_aaa111bbb222ccc333ddd444eee555" {
		t.Errorf("got %q", got)
	}
}

// Placeholder (que empieza con domk_live_ pero no tiene sufijo aleatorio) → "" (no extraer)
func TestExtractAPIKeyFromBearer_Placeholder(t *testing.T) {
	cases := []string{
		"Bearer domk_live_REEMPLAZAR_CON_TU_API_KEY",
		"Bearer domk_live_PLACEHOLDER",
		"Bearer domk_live_xxx",
		"Bearer domk_live_aaaaaaaaaaaaaaaaaaaaaaaaaaaa", // sin digitos
		"Bearer domk_live_AAAAAAAAAAAAAAAAAAAAAAAAAAAA", // sin minuscula
	}
	for _, h := range cases {
		if got := extractAPIKeyFromBearer(h); got != "" {
			t.Errorf("placeholder %q debería dar '', got %q", h, got)
		}
	}
}

// Sin prefijo Bearer → ""
func TestExtractAPIKeyFromBearer_NoPrefix(t *testing.T) {
	if got := extractAPIKeyFromBearer("domk_live_abc"); got != "" {
		t.Errorf("sin Bearer debería dar '', got %q", got)
	}
	if got := extractAPIKeyFromBearer(""); got != "" {
		t.Errorf("vacío debería dar ''")
	}
}

// writeClientEnv escribe el .env y chmod 600
func TestWriteClientEnv_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.env")
	if err := writeClientEnv(path, "domk_live_TEST"); err != nil {
		t.Fatalf("writeClientEnv: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Error(".env quedó vacío")
	}
	// chmod 600 → solo owner read/write
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}
	// Contenido debe contener la key
	body, _ := os.ReadFile(path)
	if !contains(body, "domk_live_TEST") {
		t.Errorf("body no contiene la key: %s", body)
	}
}

// Idempotente: si .env ya tiene la misma key, no falla
func TestWriteClientEnv_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	writeClientEnv(path, "domk_live_AAA")
	firstInfo, _ := os.Stat(path)

	// segunda llamada con misma key → no debe romper
	if err := writeClientEnv(path, "domk_live_AAA"); err != nil {
		t.Errorf("segunda llamada: %v", err)
	}
	secondInfo, _ := os.Stat(path)
	// mtime puede cambiar pero el contenido debe seguir OK
	body, _ := os.ReadFile(path)
	if !contains(body, "domk_live_AAA") {
		t.Errorf("contenido se corrompió: %s", body)
	}
	_ = firstInfo
	_ = secondInfo
}

// Helper: contains sin importar strings para no aumentar ruido
func contains(haystack []byte, needle string) bool {
	return stringContains(string(haystack), needle)
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// readAPIKeyFromConfig: extrae key válida de un JSON existente
func TestReadAPIKeyFromConfig_ValidOpenCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	body := `{"mcp":{"domain-mcp":{"type":"remote","headers":{"Authorization":"Bearer domk_live_abc123def456ghi789jkl012mno"}}}}`
	os.WriteFile(path, []byte(body), 0o644)

	got := readAPIKeyFromConfig(path, "mcp", "domain-mcp")
	want := "domk_live_abc123def456ghi789jkl012mno"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Mismo para claude-code (container "mcpServers")
func TestReadAPIKeyFromConfig_ValidClaudeCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")
	body := `{"mcpServers":{"domain-mcp":{"type":"http","headers":{"Authorization":"Bearer domk_live_xyz999zzz888yyy777xxx666"}}}}`
	os.WriteFile(path, []byte(body), 0o644)

	got := readAPIKeyFromConfig(path, "mcpServers", "domain-mcp")
	want := "domk_live_xyz999zzz888yyy777xxx666"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// File no existe → "" (sin error)
func TestReadAPIKeyFromConfig_NotExist(t *testing.T) {
	if got := readAPIKeyFromConfig("/nope/no/existe.json", "mcp", "domain-mcp"); got != "" {
		t.Errorf("expected '', got %q", got)
	}
}

// JSON corrupto → "" (caller puede decidir prompt al usuario)
func TestReadAPIKeyFromConfig_Corrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	os.WriteFile(path, []byte(`{"mcp":{corrupto`), 0o644)
	if got := readAPIKeyFromConfig(path, "mcp", "domain-mcp"); got != "" {
		t.Errorf("expected '', got %q", got)
	}
}

// Sin entry domain-mcp → ""
func TestReadAPIKeyFromConfig_NoEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	os.WriteFile(path, []byte(`{"mcp":{"other":{}}}`), 0o644)
	if got := readAPIKeyFromConfig(path, "mcp", "domain-mcp"); got != "" {
		t.Errorf("expected '', got %q", got)
	}
}

// Key placeholder en el JSON → "" (rechazada por extractAPIKeyFromBearer)
func TestReadAPIKeyFromConfig_PlaceholderInJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	body := `{"mcp":{"domain-mcp":{"headers":{"Authorization":"Bearer domk_live_REEMPLAZAR_CON_TU_API_KEY"}}}}`
	os.WriteFile(path, []byte(body), 0o644)
	if got := readAPIKeyFromConfig(path, "mcp", "domain-mcp"); got != "" {
		t.Errorf("placeholder debería dar '', got %q", got)
	}
}

// resolveAPIKey: flag explícito gana sobre configs
func TestResolveAPIKey_ExplicitFlagWins(t *testing.T) {
	dir := t.TempDir()
	// Aunque haya configs con keys, el flag manda
	oc := filepath.Join(dir, "opencode.json")
	cc := filepath.Join(dir, ".claude.json")
	os.WriteFile(oc, []byte(`{"mcp":{"domain-mcp":{"headers":{"Authorization":"Bearer domk_live_opencode_key_a1b2c3d4e5f6"}}}}`), 0o644)

	got, src, err := resolveAPIKey(oc, cc, "domk_live_explicit_flag1234567890123456", nil, false)
	if err != nil || got != "domk_live_explicit_flag1234567890123456" || src != "flag --api-key" {
		t.Errorf("flag debería ganar: got=%q src=%q err=%v", got, src, err)
	}
}

// resolveAPIKey: OpenCode tiene key → usa esa (sin prompt)
func TestResolveAPIKey_FromOpenCode(t *testing.T) {
	dir := t.TempDir()
	oc := filepath.Join(dir, "opencode.json")
	os.WriteFile(oc, []byte(`{"mcp":{"domain-mcp":{"headers":{"Authorization":"Bearer domk_live_from_opencode_a1b2c3d4e5f6"}}}}`), 0o644)

	got, src, err := resolveAPIKey(oc, "", "", nil, false)
	if err != nil || got != "domk_live_from_opencode_a1b2c3d4e5f6" {
		t.Errorf("opencode debería proveer: got=%q src=%q err=%v", got, src, err)
	}
	if src != "opencode" {
		t.Errorf("src = %q, want 'opencode'", src)
	}
}

// resolveAPIKey: OpenCode vacío, Claude Code tiene → fallback a Claude
func TestResolveAPIKey_FallbackToClaude(t *testing.T) {
	dir := t.TempDir()
	oc := filepath.Join(dir, "opencode-noexiste.json") // no existe
	cc := filepath.Join(dir, ".claude.json")
	os.WriteFile(cc, []byte(`{"mcpServers":{"domain-mcp":{"headers":{"Authorization":"Bearer domk_live_from_claudecode_b1c2d3e4f5a6"}}}}`), 0o644)

	got, src, err := resolveAPIKey(oc, cc, "", nil, false)
	if err != nil || got != "domk_live_from_claudecode_b1c2d3e4f5a6" || src != "claudecode" {
		t.Errorf("claudecode debería ser fallback: got=%q src=%q err=%v", got, src, err)
	}
}

// resolveAPIKey: ambas DIFERENTES → prioriza OpenCode + source indica el warning
func TestResolveAPIKey_DifferentKeys_PrioritizesOpenCode(t *testing.T) {
	dir := t.TempDir()
	oc := filepath.Join(dir, "opencode.json")
	cc := filepath.Join(dir, ".claude.json")
	os.WriteFile(oc, []byte(`{"mcp":{"domain-mcp":{"headers":{"Authorization":"Bearer domk_live_opencode_one_a1b2c3d4e5"}}}}`), 0o644)
	os.WriteFile(cc, []byte(`{"mcpServers":{"domain-mcp":{"headers":{"Authorization":"Bearer domk_live_claudecode_two_z9y8x7w6v5"}}}}`), 0o644)

	got, src, err := resolveAPIKey(oc, cc, "", nil, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "domk_live_opencode_one_a1b2c3d4e5" {
		t.Errorf("debe priorizar OpenCode, got %q", got)
	}
	if !strings.Contains(src, "opencode=") || !strings.Contains(src, "distinto") {
		t.Errorf("source debe advertir la diferencia, got %q", src)
	}
}

// resolveAPIKey: ninguna tiene key + nonInteractive → error
func TestResolveAPIKey_NoKeyNonInteractive(t *testing.T) {
	dir := t.TempDir()
	oc := filepath.Join(dir, "opencode.json")
	cc := filepath.Join(dir, ".claude.json")
	// ni opencode ni claudecode existen

	_, _, err := resolveAPIKey(oc, cc, "", nil, true)
	if err == nil {
		t.Error("debería fallar sin key y nonInteractive")
	}
}

// resolveAPIKey: ninguna tiene key + interactive → prompt (simulamos input)
func TestResolveAPIKey_PromptInteractive(t *testing.T) {
	dir := t.TempDir()
	oc := filepath.Join(dir, "opencode.json")
	cc := filepath.Join(dir, ".claude.json")

	in := bufio.NewReader(strings.NewReader("domk_live_prompted_key_z9y8x7w6v5u4\n"))
	got, src, err := resolveAPIKey(oc, cc, "", in, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "domk_live_prompted_key_z9y8x7w6v5u4" {
		t.Errorf("got %q", got)
	}
	if src != "prompt" {
		t.Errorf("src = %q, want 'prompt'", src)
	}
}

// mask: no expone el medio de la key
func TestMask(t *testing.T) {
	if got := mask("domk_live_18vqUCiHEqI53kuUZYx4Y76agbZg_rYVs6dN5l0XA3I"); got != "domk_live_...XA3I" {
		t.Errorf("got %q", got)
	}
	if got := mask("short"); got != "***" {
		t.Errorf("got %q", got)
	}
}