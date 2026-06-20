// issue-31.1 mcp-http-vps-mode — tests del handler HTTP /mcp.
//
// Cubre:
//   - Sin header Authorization → 401 con body uniforme.
//   - Header con formato invalido → 401.
//   - Token resuelve a principal pero el cliente no envia request MCP
//     valido → respuesta JSON-RPC error (lo emite mcp-go), pero el codigo
//     HTTP indica que el handler PASO la fase de auth (delegacion al
//     StreamableHTTPServer ocurrio).
//   - Initialize MCP completo → handler responde con server info que
//     incluye los tools `domain_*` registrados.

package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nunezlagos/domain/internal/auth/apikey"
	mcptools "nunezlagos/domain/internal/mcp/server"
)

// fakeResolver implementa apikey.Resolver para tests sin DB.
type fakeResolver struct {
	want      string
	principal *apikey.Principal
}

func (f *fakeResolver) Resolve(ctx context.Context, plaintext string) (*apikey.Principal, error) {
	if plaintext != f.want {
		return nil, errors.New("not found")
	}
	return f.principal, nil
}

// validAPIKey es un token con formato correcto domk_test_... 16+ chars.
const validAPIKey = "domk_test_abcdefghijklmnopqrstuvwxyz0123456789ABCDEF"

func newTestHandler() (*Handler, *fakeResolver) {
	resolver := &fakeResolver{
		want: validAPIKey,
		principal: &apikey.Principal{
			UserID:         "11111111-1111-1111-1111-111111111111",
			OrganizationID: "22222222-2222-2222-2222-222222222222",
			APIKeyID:       "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			Role:           "user",
		},
	}
	// Builder con Deps vacios (servicios nil). Para tests de auth + boot
	// del MCP basta: registramos las tools (que solo necesitan los builders
	// estaticos, no los services) y respondemos initialize/tools/list. Los
	// handlers concretos no se invocan en estos tests.
	builder := &Builder{Base: mcptools.Deps{
		ServerName: "domain-mcp-http-test",
		ServerVer:  "test",
	}}
	return NewHandler(builder, resolver), resolver
}

func TestHandler_MissingAuthorization_Returns401(t *testing.T) {
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Errorf("WWW-Authenticate header missing")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	errBlock, _ := body["error"].(map[string]any)
	if errBlock == nil || errBlock["code"] != "unauthorized" {
		t.Errorf("expected error.code=unauthorized, got body=%s", rec.Body.String())
	}
}

func TestHandler_InvalidBearerFormat_Returns401(t *testing.T) {
	h, _ := newTestHandler()
	cases := []struct {
		name, header string
	}{
		{"empty bearer", "Bearer "},
		{"non-api-key token", "Bearer not-a-valid-domk-key"},
		{"wrong scheme", "Basic dXNlcjpwYXNz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
			req.Header.Set("Authorization", tc.header)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status: got %d, want 401 (case: %s)", rec.Code, tc.name)
			}
		})
	}
}

func TestHandler_InvalidToken_Returns401(t *testing.T) {
	h, _ := newTestHandler()
	// Token con formato correcto pero que el resolver rechaza.
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer domk_test_zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
}

func TestHandler_ValidToken_InitializeReturnsServerInfo(t *testing.T) {
	h, _ := newTestHandler()
	// Request MCP initialize. mcp-go responde 200 con el server info +
	// capabilities, lo que demuestra que (a) el auth paso y (b) el MCP
	// server fue construido + delegado.
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"0.0.1"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+validAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	// La respuesta puede ser SSE o JSON segun headers; basta con que
	// contenga el nombre del servidor + alguna mention de tools en la
	// section capabilities.
	raw, _ := io.ReadAll(rec.Body)
	out := string(raw)
	if !strings.Contains(out, "domain-mcp-http-test") {
		t.Errorf("response missing serverName, got: %s", out)
	}
	if !strings.Contains(out, "tools") {
		t.Errorf("response missing tools capability, got: %s", out)
	}
}

func TestHandler_ValidToken_ToolsListReturnsDomainTools(t *testing.T) {
	h, _ := newTestHandler()
	// Pedimos tools/list — la respuesta debe incluir varios tools con prefijo domain_*.
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+validAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	// Al menos uno de los tools core deberia aparecer en el listado.
	for _, expect := range []string{"domain_mem_save", "domain_mem_search"} {
		if !strings.Contains(out, expect) {
			t.Errorf("tools/list does not contain %s — got: %s", expect, out)
		}
	}
}
