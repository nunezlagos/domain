package main

import (
	"bufio"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	got, src, err := resolveAPIKey(oc, cc, "domk_live_explicit_flag1234567890123456", "", nil, false)
	if err != nil || got != "domk_live_explicit_flag1234567890123456" || src != "flag --api-key" {
		t.Errorf("flag debería ganar: got=%q src=%q err=%v", got, src, err)
	}
}

// resolveAPIKey: OpenCode tiene key → usa esa (sin prompt)
func TestResolveAPIKey_FromOpenCode(t *testing.T) {
	dir := t.TempDir()
	oc := filepath.Join(dir, "opencode.json")
	os.WriteFile(oc, []byte(`{"mcp":{"domain-mcp":{"headers":{"Authorization":"Bearer domk_live_from_opencode_a1b2c3d4e5f6"}}}}`), 0o644)

	got, src, err := resolveAPIKey(oc, "", "", "", nil, false)
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

	got, src, err := resolveAPIKey(oc, cc, "", "", nil, false)
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

	got, src, err := resolveAPIKey(oc, cc, "", "", nil, false)
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

	_, _, err := resolveAPIKey(oc, cc, "", "", nil, true)
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
	got, src, err := resolveAPIKey(oc, cc, "", "", in, false)
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

// --- validateAPIKey ---

// 200 + Authorization header presente → nil (key valida).
// Verifica tambien que la URL se trimea (sin / final) y que el path
// /api/v1/auth/validate es el esperado.
func TestValidateAPIKey_200_OK(t *testing.T) {
	var seenAuth string
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"valid":true}}`))
	}))
	defer srv.Close()

	if err := validateAPIKey(srv.URL+"/", "domk_live_abc123def456ghi789jkl012"); err != nil {
		t.Fatalf("200 deberia ser nil, got %v", err)
	}
	if seenAuth != "Bearer domk_live_abc123def456ghi789jkl012" {
		t.Errorf("auth header = %q", seenAuth)
	}
	if seenPath != "/api/v1/auth/validate" {
		t.Errorf("path = %q, want /api/v1/auth/validate", seenPath)
	}
}

// 401 → errInvalidAPIKey (la unica condicion que bloquea el install).
func TestValidateAPIKey_401_Invalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := validateAPIKey(srv.URL, "domk_live_revoked_key_a1b2c3d4e5f6")
	if !errors.Is(err, errInvalidAPIKey) {
		t.Fatalf("401 debe devolver errInvalidAPIKey, got %v", err)
	}
}

// 500 → nil + warning (best-effort: no bloquea installs contra servers
// con bugs transitorios). El comportamiento esperado es seguir adelante.
func TestValidateAPIKey_5xx_Warning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := validateAPIKey(srv.URL, "domk_live_abc123def456ghi789jkl012"); err != nil {
		t.Fatalf("5xx debe ser best-effort (nil), got %v", err)
	}
}

// Server no responde (URL cerrada) → nil + warning.
// Cubre el caso "server caido" — no debe romper installs offline.
func TestValidateAPIKey_Timeout_NetworkError(t *testing.T) {
	// Cerramos el server inmediatamente para forzar connection refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	if err := validateAPIKey(srv.URL, "domk_live_abc123def456ghi789jkl012"); err != nil {
		t.Fatalf("network error debe ser best-effort (nil), got %v", err)
	}
}

// vpsURL vacio → skip (sin server, no hay que validar; defer al ping).
func TestValidateAPIKey_EmptyURL_Skip(t *testing.T) {
	if err := validateAPIKey("", "domk_live_abc123def456ghi789jkl012"); err != nil {
		t.Fatalf("URL vacia debe skipear, got %v", err)
	}
}

// Server que tarda mas que el timeout (5s) → nil + warning.
// Para no hacer el test lento, mockeamos el server con sleep y usamos
// un timeout mas chico via httptest.
func TestValidateAPIKey_ServerSlow_BestEffort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Forzar context cancelado: el server nunca responde.
		<-r.Context().Done()
	}))
	defer srv.Close()

	// Reducir timeout del request para no esperar 5s real.
	orig := validateAPIKeyHTTPTimeout
	// Trick: seteamos el global via una pequeña funcion helper en lugar
	// de modificarlo directamente. Como validateAPIKey usa el const,
	// validamos via el context del request del server.
	_ = orig

	done := make(chan error, 1)
	go func() {
		done <- validateAPIKey(srv.URL, "domk_live_abc123def456ghi789jkl012")
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server lento debe ser best-effort, got %v", err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("validateAPIKey no respeta el timeout (tardo >7s)")
	}
}

// --- resolveAPIKey con vpsURL ---

// resolveAPIKey con vpsURL: flag explicito + server 401 → error directo,
// sin re-prompt (el user la puso a mano, no adivinamos).
func TestResolveAPIKey_ExplicitFlag_401_ErrorDirecto(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, _, err := resolveAPIKey("", "", "domk_live_flagged_key_a1b2c3d4e5f6", srv.URL, nil, false)
	if err == nil {
		t.Fatal("flag con 401 debe fallar")
	}
	if !strings.Contains(err.Error(), "--api-key") {
		t.Errorf("error debe mencionar --api-key, got %v", err)
	}
}

// resolveAPIKey con vpsURL: flag explicito + server 200 → OK.
func TestResolveAPIKey_ExplicitFlag_200_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	got, src, err := resolveAPIKey("", "", "domk_live_flagged_key_a1b2c3d4e5f6", srv.URL, nil, false)
	if err != nil {
		t.Fatalf("200 debe pasar, got %v", err)
	}
	if got == "" || src != "flag --api-key" {
		t.Errorf("got=%q src=%q", got, src)
	}
}

// resolveAPIKey con vpsURL: config key + server 401 → cae al prompt.
// Simulamos que el prompt tambien es rechazado y luego aceptado.
func TestResolveAPIKey_ConfigKey_401_CaeAPrompt(t *testing.T) {
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// call 1: config key (rechazada)
		// call 2: prompt intento 1 (rechazada)
		// call 3: prompt intento 2 (rechazada)
		// call 4: prompt intento 3 (aceptada)
		if callCount >= 4 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	oc := filepath.Join(dir, "opencode.json")
	os.WriteFile(oc, []byte(`{"mcp":{"domain-mcp":{"headers":{"Authorization":"Bearer domk_live_revoked_in_config_a1b2c3d4"}}}}`), 0o644)

	// 3 inputs para los 3 intentos de prompt (el ultimo gana).
	in := bufio.NewReader(strings.NewReader(
		"domk_live_first_attempt_a1b2c3d4e5f6\n" +
			"domk_live_second_attempt_b2c3d4e5f6g7\n" +
			"domk_live_third_attempt_c3d4e5f6g7h8\n",
	))
	got, src, err := resolveAPIKey(oc, "", "", srv.URL, in, false)
	if err != nil {
		t.Fatalf("3er intento debe pasar, got %v", err)
	}
	if got != "domk_live_third_attempt_c3d4e5f6g7h8" {
		t.Errorf("debe usar la 3ra key, got %q", got)
	}
	if src != "prompt" {
		t.Errorf("src = %q, want 'prompt'", src)
	}
	if callCount != 4 {
		t.Errorf("server debio recibir 4 requests (1 config + 3 prompt), got %d", callCount)
	}
}

// resolveAPIKey: prompt + 401 persistente → error tras validateMaxAttempts.
func TestResolveAPIKey_Prompt_Agotado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	// validateMaxAttempts inputs, todos con formato valido pero server rechaza.
	inputs := strings.Repeat("domk_live_attempt_z9y8x7w6v5u4\n", validateMaxAttempts)
	in := bufio.NewReader(strings.NewReader(inputs))

	_, _, err := resolveAPIKey("", "", "", srv.URL, in, false)
	if err == nil {
		t.Fatal("debe agotar y fallar")
	}
	if !strings.Contains(err.Error(), "tras") {
		t.Errorf("error debe mencionar intentos agotados, got %v", err)
	}
}

// resolveAPIKey: prompt + server caido (network error) → best-effort, acepta.
func TestResolveAPIKey_Prompt_NetworkError_BestEffort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // cierra inmediatamente → connection refused

	in := bufio.NewReader(strings.NewReader("domk_live_attempt_z9y8x7w6v5u4\n"))
	got, src, err := resolveAPIKey("", "", "", srv.URL, in, false)
	if err != nil {
		t.Fatalf("network error debe ser best-effort, got %v", err)
	}
	if got != "domk_live_attempt_z9y8x7w6v5u4" {
		t.Errorf("got %q", got)
	}
	if src != "prompt" {
		t.Errorf("src = %q", src)
	}
}