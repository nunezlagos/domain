# HU-12.1-mcp-core-stdio

**Origen:** `REQ-12-mcp-server`
**Persona:** dx-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** plataforma Domain
**Quiero** implementar un servidor MCP (Model Context Protocol) vía stdio usando `mark3labs/mcp-go`
**Para** exponer todas las capacidades de la plataforma como tools MCP que los LLM y agentes puedan invocar

## Criterios de aceptación

### Escenario 1: Servidor inicia y responde a initialize

```gherkin
Dado el binario `domain-mcp` compilado
Cuando un cliente MCP (como Claude Desktop) lo invoca con `initialize`
Entonces el servidor responde con un mensaje JSON-RPC `initialize` exitoso
Y el servidor declara:
  | protocol_version | "2025-03-26" |
  | capabilities     | {tools:{}}   |
  | server_name      | "domain-mcp"|
  | server_version   | "0.1.0"      |
```

### Escenario 2: Tool registration y listado

```gherkin
Dado que el servidor MCP está inicializado
Cuando el cliente envía `tools/list`
Entonces el servidor responde con la lista de tools registradas
Y cada tool tiene:
  | name        | string  |
  | description | string  |
  | inputSchema | object  |
Y al menos las tools de memoria están presentes
```

### Escenario 3: Invocación de tool con éxito

```gherkin
Dado el servidor MCP inicializado con memoria_tools registrados
Cuando el cliente envía `tools/call` con:
  | name       | "domain_mem_search"                |
  | arguments  | {"query":"arquitectura"}    |
Entonces el servidor ejecuta la tool
Y responde con un mensaje `tools/call` exitoso
Y el contenido incluye los resultados de búsqueda
```

### Escenario 4: Invocación con error retorna error envelope

```gherkin
Dado el servidor MCP inicializado
Cuando el cliente envía `tools/call` con nombre de tool inexistente
Entonces el servidor responde con un mensaje JSON-RPC error
Y el código de error es `-32601` (Method not found)
Y el mensaje contiene `Unknown tool: tool_inexistente`
```

### Escenario 5: Argumentos inválidos son rechazados

```gherkin
Dado la tool `domain_mem_save` que requiere los argumentos "title" y "content"
Cuando el cliente envía `tools/call` con:
  | name      | "domain_mem_save"       |
  | arguments | {"title":"solo"} |
Entonces el servidor responde con error `-32602` (Invalid params)
Y el mensaje indica que falta el argumento "content"
```

### Escenario 6: Graceful shutdown

```gherkin
Dado el servidor MCP corriendo
Cuando recibe `SIGTERM`
Entonces el servidor completa la tool en ejecución actual (con timeout de 5s)
Y cierra todas las conexiones
Y termina el proceso con código 0

Cuando recibe `SIGINT` (Ctrl+C)
Entonces el servidor cancela la tool en ejecución
Y termina inmediatamente con código 0
```

### Escenario 7: JSON-RPC message handling

```gherkin
Dado el servidor MCP recibiendo mensajes por stdin
Cuando recibe un mensaje JSON-RPC válido
Entonces lo parsea correctamente
Y enruta al handler correspondiente
Y envía la respuesta por stdout

Cuando recibe un mensaje malformado (JSON inválido)
Entonces responde con error `-32700` (Parse error)
Y continúa escuchando siguientes mensajes
```

## Análisis breve

- **Qué pide realmente:** Core del servidor MCP sobre stdio usando `mark3labs/mcp-go`: inicialización, tool registration, JSON-RPC message handling, errores y graceful shutdown. Es la base sobre la cual se registran las tools específicas (HU-12.2, HU-12.3).
- **Módulos sospechados:** `cmd/domain-mcp/`, `internal/mcp/server/`, `internal/mcp/handler/`
- **Riesgos / dependencias:** `mark3labs/mcp-go` puede tener breaking changes. El protocolo MCP está en evolución (2025-03-26).
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
