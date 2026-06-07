# Proposal: HU-04.1-mcp-core

## Intención

Construir la infraestructura base del servidor MCP stdio: transporte, ciclo JSON-RPC, registro de todas las tools, errores estandarizados y shutdown graceful. Es el cimiento sobre el que se montan las 18 tools restantes.

## Scope

**Incluye:**
- Transporte stdio (stdin lectura línea a línea como JSON-RPC, stdout para respuestas, stderr para logging)
- Inicialización MCP (handshake `initialize` / `initialized`)
- Registro del handler genérico de `tools/list` y `tools/call`
- Registro de las 19 tools con name, description, inputSchema (el cuerpo del handler se implementa en HUs posteriores — esta HU deja stubs que devuelven error "not implemented")
- Manejo de errores: ParseError, InvalidRequest, MethodNotFound, InternalError, ToolNotFound
- Captura de señales SIGTERM/SIGINT para shutdown graceful
- Logging a stderr con prefijo `[mcp]`

**Excluye:**
- Implementación de la lógica de negocio de las tools (cada HU posterior)
- Soporte de transporte HTTP/WebSocket (solo stdio)
- Soporte de notificaciones/progress (futuro)

## Enfoque técnico

1. **Dependencia:** `github.com/mark3labs/mcp-go` v0.8+ para tipos MCP y helpers de server.
2. **Estructura:** `cmd/mem/mcp.go` o flag `mem mcp` que arranca el server.
3. **Paquete interno:** `internal/mcp/server.go` con `func Run(ctx context.Context, opts ...Option) error`.
4. **Tool registry pattern:** Mapa `map[string]ToolDefinition` con name, description, inputSchema, y un handler `func(ctx context.Context, req mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error)`. Las tools no implementadas devuelven `ErrNotImplemented`.
5. **Shutdown:** `signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)`. El server cierra el transporte y espera hasta 5s a requests en curso via `sync.WaitGroup`.
6. **Logging:** `slog.New(slog.NewTextHandler(os.Stderr, nil))` con prefijo de source.

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| `mark3labs/mcp-go` cambia API | Bloqueante | Pin a v0.8, wrapper thin |
| Cliente envía request enorme (DOS) | Memory pressure | Límite de 1MB por mensaje |
| Shutdown lento (>5s) | Process kill | Timeout duro con `context.WithTimeout` |
| Stderr logging rompe cliente MCP | Confusión | Nunca escribir a stdout fuera de JSON-RPC |

## Testing

- **Unit:** Test de parseo JSON-RPC, error codes, tool registry, signal handling (simulado)
- **Integration:** Lanzar binary como subprocess, pipe stdin/stdout, probar initialize, tools/list, tools/call, y errores
- **Sabotaje:** Enviar binary garbage → esperar ParseError. Enviar tool name vacío → esperar error.
