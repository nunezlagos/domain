# Design: issue-04.1-mcp-core

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Librería MCP | `mark3labs/mcp-go` | SDK oficial de Anthropic (Go bindings), homemade JSON-RPC |
| Transporte | stdio | HTTP SSE, WebSocket |
| Tool registry | `map[string]ToolHandler` con reflexión opcional | Code-gen from proto, interface-based |
| Shutdown | `signal.NotifyContext` + `sync.WaitGroup` | Grace period configurable |

Se elige `mark3labs/mcp-go` porque es la librería Go más madura para MCP, usada en producción por múltiples proyectos. Stdio porque es el transporte requerido para Claude Desktop y Cline. El registry con mapa permite registro diferido de tools (cada HU registra sus handlers sin tocar el core).

## Alternativas descartadas

- **MCP SDK oficial de Anthropic (Python/TS):** No existe SDK Go oficial. `mark3labs/mcp-go` es el estándar de facto.
- **HTTP SSE:** Más complejo, requeriría servidor HTTP, puerto, CORS. Claude Desktop no lo soporta para servers locales.
- **Homemade JSON-RPC:** Reinventar la rueda. La librería ya maneja versioning, batching, errores estandarizados.

## Diagrama

```
┌─────────────────────────────────────────────────┐
│                   Client (MCP)                   │
│  Claude Desktop / Cline / custom agent           │
└──────────────┬──────────────────────┬───────────┘
               │ stdin (JSON-RPC req)  │ stdout (resp)
               ▼                      ▲
┌─────────────────────────────────────────────────┐
│              memoria MCP Server                  │
│                                                   │
│  ┌──────────┐  ┌──────────────────────────────┐  │
│  │ Transport │  │       Tool Registry           │  │
│  │ stdio     │──│  map[string]ToolHandler       │  │
│  │ line buf  │  │                              │  │
│  └──────────┘  │  - domain_mem_save (stub → issue-04.2)  │  │
│       │        │  - domain_mem_search (stub → issue-04.3) │  │
│       │        │  - ...                        │  │
│       │        │  - mem_merge_projects (stub)   │  │
│       │        └──────────────────────────────┘  │
│       │                                           │
│  ┌────┴─────┐  ┌──────────────────────────────┐  │
│  │ Signal   │  │  Error Handler                │  │
│  │ Handler  │  │  ParseError → -32700          │  │
│  │ SIGTERM  │  │  InvalidReq → -32600          │  │
│  │ SIGINT   │  │  NotFound → -32601            │  │
│  └──────────┘  │  Internal → -32603            │  │
│                └──────────────────────────────┘  │
│  ┌──────────────────────────────────────────────┐ │
│  │ Logger (slog → stderr)                      │ │
│  └──────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

## TDD plan

**Red:**
1. Test `TestInitialize` envía initialize → recibe serverInfo con name "Domain"
2. Test `TestToolList` pide tools/list → recibe array con >= 19 tools
3. Test `TestToolCallValid` llama mem_current_project → isError = false
4. Test `TestToolCallUnknown` llama tool inexistente → isError = true
5. Test `TestParseError` envía `{invalid` → código -32700
6. Test `TestInvalidRequest` envía `{"a":1}` → código -32600
7. Test `TestGracefulShutdown` envía SIGTERM → server termina con código 0

**Green:** Implementación mínima con stubs.

**Refactor:** Extraer registry pattern, logging configurable, opciones de server.

**Sabotaje:** Romper parseo → confirmar que test ParseError falla. Restaurar.

## Riesgos y mitigación

- **Lock de stdin por lectura bloqueante:** Usar `bufio.Scanner` con timeout. Si el cliente cierra stdin, scanner retorna EOF y server hace shutdown limpio.
- **Dependencia externa `mcp-go`:** Pin commit SHA en `go.mod` además de tag. CI corre con `go mod verify`.
- **Memory leak en tool registry:** Solo registro, no crece en runtime. Tools con estado (sesiones) se gestionan fuera del core.
