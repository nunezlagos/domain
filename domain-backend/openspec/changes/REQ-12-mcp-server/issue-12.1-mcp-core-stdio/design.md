# Design: issue-12.1-mcp-core-stdio

## Decisión arquitectónica

```
┌──────────────────────────────────────────────────────────┐
│                   Cliente MCP (Claude, etc)               │
│                      │  stdin/stdout                      │
│                      │  JSON-RPC 2.0                      │
└──────────────────────┼───────────────────────────────────┘
                       │
┌──────────────────────┼───────────────────────────────────┐
│              domain-mcp (cmd/domain-mcp)                │
│                       │                                   │
│              ┌────────┴────────┐                          │
│              │  MCPServer      │                          │
│              │  (mcp-go)       │                          │
│              │                 │                          │
│              │  Initialize()   │                          │
│              │  ListTools()    │                          │
│              │  CallTool()     │                          │
│              └────────┬────────┘                          │
│                       │                                   │
│              ┌────────┴────────┐                          │
│              │  ToolRegistry   │                          │
│              │                 │                          │
│              │  map[string]    │                          │
│              │  ToolHandler    │                          │
│              │                 │                          │
│              │  RegisterTool() │                          │
│              │  GetTool()      │                          │
│              └─────────────────┘                          │
│                       │                                   │
│              ┌────────┴────────┐                          │
│              │ ServiceRegistry │                          │
│              │ (DI container)  │                          │
│              │                 │                          │
│              │ MemoryService   │                          │
│              │ AgentService    │                          │
│              │ FlowService     │                          │
│              │ SkillService    │                          │
│              │ KnowledgeService│                          │
│              │ CronService     │                          │
│              └─────────────────┘                          │
└───────────────────────────────────────────────────────────┘
```

**Decisión:** Usar `mark3labs/mcp-go` directamente como librería core del protocolo. No abstraer sobre ella a menos que sea necesario para testing. ToolRegistry como mapa de handlers con validación de esquema. ServiceRegistry como DI simple para que las tools accedan a servicios de plataforma.

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|---|---|
| Implementar JSON-RPC desde cero | mark3labs/mcp-go ya maneja initialize, ping, errores, transporte stdio |
| Usar modelo de plugins (hashicorp/go-plugin) | Overkill, MCP ya es el protocolo de plugin |
| Transporte HTTP en lugar de stdio | stdio es lo que Claude Desktop y la mayoría de clientes MCP esperan |
| gRPC como transporte MCP | MCP define stdio y SSE, no gRPC |

## Diagrama

```
─── CICLO DE VIDA DEL SERVIDOR MCP ───

1. Inicio: main() → logger setup → signal handling → NewMCPServer()
2. Registro: RegisterTool("domain_mem_save", ...) → RegisterTool("domain_mem_search", ...) → ...
3. Serve: server.ServeStdio() → bloquea leyendo stdin
4. Initialize: cliente envía initialize → server responde con info + capabilities
5. Tools/List: cliente lista tools → server responde con tools registradas
6. Tools/Call: cliente invoca tool → server valida args → ejecuta handler → responde
7. Error: si algo falla → server responde con JSON-RPC error
8. Shutdown: SIGTERM/SIGINT o stdin EOF → server.Shutdown() → exit 0

─── FORMATO DE MENSAJE STDIO ───

Cada mensaje JSON-RPC en una línea (delimitado por \n):
→ {"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}
← {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26",...}}

→ {"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
← {"jsonrpc":"2.0","id":2,"result":{"tools":[{...},{...}]}}

→ {"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"domain_mem_search","arguments":{"query":"test"}}}
← {"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"..."}]}}
```

## TDD plan

1. **Red:** Test que server responde a initialize con protocol version correcta
2. **Green:** Implementar NewMCPServer() con mcp.NewServer() mockeable
3. **Refactor:** Extraer ToolRegistry interface
4. **Red:** Test que RegisterTool + ListTools devuelve tool registrada
5. **Green:** Implementar RegisterTool usando server.AddTool()
6. **Red:** Test que CallTool con nombre existente ejecuta handler
7. **Green:** Implementar CallTool routing
8. **Red:** Test que CallTool con nombre inexistente devuelve error -32601
9. **Green:** Implementar error handling en routing
10. **Red:** Test que argumentos requeridos faltantes devuelven -32602
11. **Green:** Implementar validación de argumentos
12. **Red:** Test que stdin EOF causa shutdown graceful
13. **Green:** Implementar detección de EOF
14. **Sabotaje:** No validar args → tool recibe nil → panic

## Riesgos y mitigación

- **stdout contaminado:** Toda salida no JSON-RPC a stdout rompe el protocolo. Mitigación: reemplazar log.Default() con writer a stderr al inicio de main().
- **Buffer de stdin:** Si el cliente envía muchas líneas rápido, puede saturar. Mitigación: bufio.Scanner con buffer grande.
- **MCP version mismatch:** Cliente puede pedir versión diferente. Mitigación: negociar versión, responder con la que soportamos.
- **Dependency injection:** Muchas tools → muchos servicios. Mitigación: ServiceRegistry lazy-loaded, no inicializar todo al arranque.
