# Proposal: issue-12.1-mcp-core-stdio

## Intención

Construir el núcleo del servidor MCP de Domain sobre stdio usando la librería `mark3labs/mcp-go`. Implementar el ciclo de vida completo del protocolo: initialize, tool registration, tools/list, tools/call, errores JSON-RPC y graceful shutdown. Este es el esqueleto sobre el cual se registrarán todas las tools específicas.

## Scope

**Incluye:**
- Binary `cmd/domain-mcp/main.go` que inicia el servidor MCP
- Integración con `mark3labs/mcp-go` para manejo del protocolo
- Inicialización: responder a `initialize` con server info y capabilities
- Tool registry: `RegisterTool(name, description, schema, handler)`
- Tools/list: devolver todas las tools registradas con su inputSchema
- Tools/call: enrutar a handler según tool name, validar argumentos, ejecutar, devolver resultado
- Manejo de errores JSON-RPC estándar (-32700, -32601, -32602, -32603)
- Graceful shutdown: SIGTERM (completar y cerrar) y SIGINT (cancelar y cerrar)
- Lectura/escritura por stdin/stdout con delimitación de newline (MCP stdio transport)
- Logging a stderr (para no interferir con stdout que transporta JSON-RPC)

**No incluye:**
- Tools específicas (issue-12.2, issue-12.3)
- Transporte HTTP/SSE (solo stdio por ahora)
- Autenticación (se hereda del proceso padre)
- Multi-sesión (un proceso = una sesión MCP)

## Enfoque técnico

1. `cmd/domain-mcp/main.go`: inicializa logger (stderr), crea `MCPServer`, registra tools, llama a `ServeStdio()`
2. `internal/mcp/server/server.go`: wrapper sobre `mcp.NewServer()` de `mark3labs/mcp-go`
3. Server expone `RegisterTool(name, description, schema, handlerFunc)` que internamente llama a `server.AddTool()`
4. `handlerFunc` recibe `mcp.CallToolRequest` y devuelve `(*mcp.CallToolResult, error)`
5. El server maneja automáticamente: initialize, tools/list, tools/call, JSON-RPC parseo, errores
6. Graceful shutdown con `signal.NotifyContext` + `os.Signal`
7. Logging: usar log/slog con output a stderr, nivel configurable vía flag `--log-level`
8. Dependency injection: el server recibe un `ServiceRegistry` con acceso a servicios de la plataforma

## Riesgos

- **mark3labs/mcp-go API inestable:** El protocolo MCP está evolucionando. Mitigación: pin version, wrapper interface propia para aislar cambios.
- **stdout contaminado:** Cualquier `fmt.Print` o log a stdout rompe el protocolo. Mitigación: log.Println redirigido a stderr en entry point, linter rule.
- **Proceso padre muere:** stdin/stdout se cierran, el servidor debe detectar EOF y hacer shutdown. Mitigación: detectar stdin EOF y terminar.
- **Timeout en tools largas:** Si una tool tarda mucho, el cliente MCP puede timeout. Mitigación: las tools largas deben devolver progreso o usarse async.

## Testing

- Unit: test de registro de tools
- Unit: test de enrutamiento tools/call por nombre
- Unit: test de validación de argumentos requeridos
- Integration: iniciar server, enviar initialize por stdin, leer respuesta por stdout
- Integration: tools/list, tools/call con mock handler
- Integration: error handling (tool inexistente, argumentos inválidos)
- Integration: graceful shutdown con SIGTERM
- Integration: JSON-RPC parse error con mensaje malformado
