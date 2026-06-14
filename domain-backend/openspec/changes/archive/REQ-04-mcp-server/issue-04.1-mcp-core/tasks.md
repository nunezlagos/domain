# Tasks: issue-04.1-mcp-core

## Backend

- [ ] Crear `internal/mcp/server.go`: estructura `Server`, campo `registry map[string]ToolHandler`
- [ ] Implementar constructor `New()` con opciones funcionales (`WithLogger`, `WithToolRegistrar`)
- [ ] Implementar `func (s *Server) RegisterTool(name string, def ToolDefinition)`
- [ ] Implementar `func (s *Server) Run(ctx context.Context) error`: loop stdin → parse → dispatch → stdout
- [ ] Implementar parseo JSON-RPC con `mcp-go` types (InitializeRequest, CallToolRequest, ListToolsRequest, etc.)
- [ ] Implementar `handleInitialize`: devolver serverInfo y capabilities
- [ ] Implementar `handleListTools`: iterar registry, devolver array de Tool
- [ ] Implementar `handleCallTool`: lookup en registry, ejecutar handler, devolver resultado
- [ ] Implementar error handler: `-32700 ParseError`, `-32600 InvalidRequest`, `-32601 MethodNotFound`, `-32603 InternalError`
- [ ] Registrar stubs para las 19 tools (name + description + inputSchema, handler devuelve error "not implemented yet")
- [ ] Implementar signal handler: `signal.NotifyContext(ctx, SIGTERM, SIGINT)`, graceful period 5s
- [ ] Implementar rate limit / size limit: rechazar mensajes > 1MB con error -32600
- [ ] Añadir entrada `mem mcp` en CLI (flag o subcommand en `cmd/`)
- [ ] Agregar logging a stderr con `slog`, nivel configurable vía `MCP_LOG_LEVEL`

## Tests

- [ ] Test unitario: `TestNewServer` verifica que registry está vacío
- [ ] Test unitario: `TestRegisterTool` verifica registro y colisión
- [ ] Test unitario: `TestParseError` envía JSON inválido, espera código -32700
- [ ] Test integración: lanzar server como subprocess, pipe stdin/stdout, test initialize + tools/list + tools/call
- [ ] Test integración: `TestGracefulShutdown` envía SIGTERM, espera exit 0
- [ ] Test integración: `TestToolUnknown` llama tool que no existe, verifica isError
- [ ] Sabotaje: enviar `\x00\x00` raw bytes → esperar ParseError, restaurar fix

## Cierre

- [ ] Verificación manual: `echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | mem mcp` devuelve tools
- [ ] Verificación manual: probar conexión desde Claude Desktop apuntando a `mem mcp`
- [ ] Suite verde: `go test ./internal/mcp/...`
