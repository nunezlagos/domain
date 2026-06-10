# Tasks: issue-12.1-mcp-core-stdio

## Backend

- [ ] Crear `cmd/domain-mcp/main.go` con flags --log-level, --config
- [ ] Implementar `internal/mcp/server/server.go`: wrapper sobre `mcp.NewServer()` de mark3labs/mcp-go
- [ ] Implementar `RegisterTool(name, description, schema, handler)` en MCPServer
- [ ] Implementar `ServiceRegistry` simple como DI container para servicios de plataforma
- [ ] Implementar manejo de initialize: responder con protocol version y capabilities
- [ ] Implementar manejo de tools/list: devolver todas las tools registradas
- [ ] Implementar manejo de tools/call: enrutar por nombre, validar argumentos, ejecutar handler
- [ ] Implementar validación de argumentos requeridos contra inputSchema
- [ ] Implementar error handling JSON-RPC: -32700 (parse), -32601 (method), -32602 (params), -32603 (internal)
- [ ] Implementar graceful shutdown con signal.NotifyContext (SIGTERM, SIGINT)
- [ ] Implementar detección de stdin EOF para shutdown
- [ ] Configurar logging a stderr con slog
- [ ] Configurar go.mod con dependencia `github.com/mark3labs/mcp-go`
- [ ] Crear Makefile target `build-mcp` para compilar domain-mcp

## Frontend

- [ ] (No aplica - MCP es protocolo, no UI)

## Tests

- [ ] Test unitario: NewMCPServer() no devuelve nil
- [ ] Test unitario: RegisterTool + GetTool devuelve handler registrado
- [ ] Test unitario: CallTool con nombre existente ejecuta handler
- [ ] Test unitario: CallTool con nombre inexistente devuelve error -32601
- [ ] Test unitario: CallTool con argumentos faltantes devuelve error -32602
- [ ] Test unitario: CallTool con JSON inválido devuelve error -32700
- [ ] Test unitario: graceful shutdown con señal SIGTERM
- [ ] Test unitario: stdin EOF causa shutdown
- [ ] Test integración: iniciar server por stdin/stdout pipe, enviar initialize, recibir respuesta
- [ ] Test integración: ciclo completo initialize → tools/list → tools/call
- [ ] Sabotaje: escribir log a stdout → protocolo se rompe → test detecta
- [ ] Sabotaje: no capturar SIGTERM → server no hace cleanup

## Cierre

- [ ] Verificación manual: conectar domain-mcp desde Claude Desktop
- [ ] Verificación manual: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./domain-mcp`
- [ ] Suite verde: `go test ./internal/mcp/... ./cmd/domain-mcp/...`
- [ ] Documentar configuración de Claude Desktop para conectar domain-mcp
