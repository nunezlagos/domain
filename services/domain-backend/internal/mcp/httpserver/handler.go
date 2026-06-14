// Package httpserver — issue-31.1 + 31.2 mcp-http-vps-mode.
//
// Expone las mismas tools MCP que `cmd/domain-mcp` (stdio) sobre HTTP
// Streamable Transport (MCP spec 2025+). Permite que clientes MCP
// remotos (claude-code, Cursor, Cline, etc.) se conecten al VPS via
// `https://api.tudominio.com/mcp` autenticando con Bearer API key.
//
// Diseño:
//   - El binario `domain server` instancia un Builder con todas las
//     dependencias (services + audit + pools + LLM factory) ya cableadas.
//   - El handler HTTP en /mcp valida el header Authorization: Bearer
//     <api_key>, resuelve el Principal via apikey.Resolver, construye
//     un mcpserver.Deps por request con ese Principal, monta un
//     MCPServer + StreamableHTTPServer stateless y delega ServeHTTP.
//   - Cada request es una sesion MCP independiente: stateless = true,
//     sin estado server-side entre requests del mismo cliente. El
//     overhead extra (registro de ~40 tools por request) es aceptable
//     para un endpoint multi-tenant low-volume; si fuera bottleneck,
//     el cache puede pin-ear MCPServer por principal-id.
//
// Compatibilidad: el binario `domain-mcp` (stdio) sigue funcionando
// igual; comparte 100% del codigo de tools via internal/mcp/server.
package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	mcpserver "nunezlagos/domain/internal/mcp/server"
)

// Builder produce un mcpserver.Deps por request HTTP MCP. Sostiene los
// services compartidos (que son thread-safe y stateless respecto del
// principal); solo el Principal y los wrappers dependientes de Principal
// se rearman por request.
type Builder struct {
	// Base contiene todos los punteros a services + pool + dispatcher.
	// El campo Principal de Base se ignora; se setea por request.
	Base mcpserver.Deps
}

// Resolver tipo (alias para readability) — mismo contrato que
// apikey.Middleware.Resolver / apikey.CachedResolver.
type Resolver = apikey.Resolver

// Handler crea el http.Handler que sirve MCP Streamable HTTP en /mcp.
//
// Comportamiento:
//   - Requiere header `Authorization: Bearer <api_key>`. Si falta o el
//     formato no es API key valida, responde 401 con shape uniforme
//     (mismo body que apikey.Middleware para no leakear info).
//   - Resuelve el token via resolver (puede ser CachedResolver para
//     evitar hit a Postgres por request).
//   - Construye un MCPServer con Deps clonado + Principal resuelto y
//     delega al StreamableHTTPServer stateless de mcp-go.
//   - El endpointPath del StreamableHTTPServer es "/mcp" por default,
//     que matchea la ruta donde el caller monta este handler.
type Handler struct {
	Builder  *Builder
	Resolver Resolver
}

// NewHandler crea un Handler listo para montar via http.Handle("/mcp", h).
func NewHandler(b *Builder, resolver Resolver) *Handler {
	return &Handler{Builder: b, Resolver: resolver}
}

// ServeHTTP implementa http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(header, bearerPrefix) {
		writeUnauthorized(w)
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
	if !apikey.IsAPIKeyFormat(token) {
		writeUnauthorized(w)
		return
	}
	principal, err := h.Resolver.Resolve(r.Context(), token)
	if err != nil || principal == nil {
		writeUnauthorized(w)
		return
	}

	// Clonar Deps con principal del request. Los punteros a services
	// son compartidos (thread-safe); solo Principal cambia.
	deps := h.Builder.Base
	deps.Principal = principal
	if deps.ServerName == "" {
		deps.ServerName = "domain-mcp-http"
	}

	srv := mcpserver.New(deps)
	streamable := mcpgo.NewStreamableHTTPServer(srv,
		mcpgo.WithStateLess(true),
		mcpgo.WithEndpointPath("/mcp"),
	)
	streamable.ServeHTTP(w, r)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", `Bearer realm="domain-mcp"`)
	w.WriteHeader(http.StatusUnauthorized)
	body, _ := json.Marshal(map[string]any{
		"error": map[string]string{
			"code":    "unauthorized",
			"message": "unauthorized",
		},
	})
	_, _ = w.Write(body)
}
