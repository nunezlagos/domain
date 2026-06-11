# Tasks: issue-12.1-mcp-core-stdio

## Backend

- [x] Crear `cmd/domain-mcp/main.go` con flags --log-level, --config
- [x] Implementar `internal/mcp/server/server.go`: wrapper sobre `mcp.NewServer()` de mark3labs/mcp-go
- [x] Implementar `RegisterTool(name, description, schema, handler)` en MCPServer
- [x] Implementar `ServiceRegistry` simple como DI container para servicios de plataforma
- [x] Implementar manejo de initialize: responder con protocol version y capabilities
- [x] Implementar manejo de tools/list: devolver todas las tools registradas
- [x] Implementar manejo de tools/call: enrutar por nombre, validar argumentos, ejecutar handler
- [x] Implementar validación de argumentos requeridos contra inputSchema
- [x] Implementar error handling JSON-RPC: -32700 (parse), -32601 (method), -32602 (params), -32603 (internal)
- [x] Implementar graceful shutdown con signal.NotifyContext (SIGTERM, SIGINT)
- [x] Implementar detección de stdin EOF para shutdown
- [x] Configurar logging a stderr con slog
- [x] Configurar go.mod con dependencia `github.com/mark3labs/mcp-go`
- [x] Crear Makefile target `build-mcp` para compilar domain-mcp

## Frontend

- [x] (No aplica - MCP es protocolo, no UI)

## Tests

- [x] Test unitario: NewMCPServer() no devuelve nil
- [x] Test unitario: RegisterTool + GetTool devuelve handler registrado
- [x] Test unitario: CallTool con nombre existente ejecuta handler
- [x] Test unitario: CallTool con nombre inexistente devuelve error -32601
- [x] Test unitario: CallTool con argumentos faltantes devuelve error -32602
- [x] Test unitario: CallTool con JSON inválido devuelve error -32700
- [x] Test unitario: graceful shutdown con señal SIGTERM
- [x] Test unitario: stdin EOF causa shutdown
- [x] Test integración: iniciar server por stdin/stdout pipe, enviar initialize, recibir respuesta
- [x] Test integración: ciclo completo initialize → tools/list → tools/call
- [x] Sabotaje: escribir log a stdout → protocolo se rompe → test detecta
- [x] Sabotaje: no capturar SIGTERM → server no hace cleanup

## Cierre

- [x] Verificación manual: conectar domain-mcp desde Claude Desktop
- [x] Verificación manual: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./domain-mcp`
- [x] Suite verde: `go test ./internal/mcp/... ./cmd/domain-mcp/...`
- [x] Documentar configuración de Claude Desktop para conectar domain-mcp
