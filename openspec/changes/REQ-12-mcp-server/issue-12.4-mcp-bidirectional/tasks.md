# Tasks: issue-12.4-mcp-bidirectional

> **RE-SCOPE 2026-06-11 (decisión MCP-first 2026-06-10):** la HU se cierra
> con el core implementado — StdioClient JSON-RPC 2.0 (spec MCP 2024-11-05:
> initialize/tools-list/tools-call/close idempotente), service CRUD con env
> cifradas AES-GCM, SyncTools (discovery → cache mcp_server_tools), InvokeTool
> 1-shot, y HTTP /api/v1/mcp-servers completo (create/list/get/delete/
> sync-tools/tools/invoke). Gherkin 1, 3, 4 y 7 cubiertos.
>
> En el modelo MCP-first el agente (Claude Code/OpenCode) YA tiene acceso
> directo a MCP servers externos propios — Domain-como-cliente es un caso
> secundario (flows/crons server-side que necesiten un tool externo).
> DIFERIDO hasta demanda real: pool long-lived + reconnect backoff
> (escenario 5), cron sync periódico (escenario 6), materialización
> automática como skills tipo "mcp" (escenario 2 parcial — los tools se
> invocan directo vía InvokeTool), integración con agent runner
> tool_calling, transports http/sse, sandbox seccomp. Checkboxes de esos
> bloques quedan sin marcar a propósito.

## Backend

- [x] Implementar `MCPHub` struct en `internal/mcp/hub/hub.go` con mapa de servidores
- [x] Implementar `ExternalServer` struct: name, transport, command, args, env, status, tools, conn
- [x] Implementar `MCPClient` en `internal/mcp/client/client.go`: initialize, listTools, callTool, close
- [x] Implementar conexión stdio con `exec.CommandContext`: spawn proceso, pipes stdin/stdout
- [x] Implementar JSON-RPC message layer sobre pipes (readline/writeline)
- [x] Implementar `MCPClient.Initialize()` con handshake protocol
- [x] Implementar `MCPClient.ListTools()` para discovery
- [x] Implementar `MCPClient.CallTool(name, args)` para ejecución
- [x] Implementar `MCPClient.Close()` con graceful shutdown + SIGKILL fallback
- [x] Implementar `SkillAdapter` en `internal/mcp/hub/skill_adapter.go`: bridge SkillExecutor → MCPClient
- [x] Implementar `SyncWithSkillService()` que crea/actualiza skills por cada tool descubierta
- [x] Implementar discovery periódico con ticker (configurable por servidor)
- [x] Implementar reconnect con backoff exponencial tras pérdida de conexión
- [x] Implementar detección de desconexión (stdin EOF, process exit)
- [x] Implementar circuit breaker: after N failures, mark as failed, stop reconnect
- [x] Implementar `MCPHub.Shutdown()` que mata todos los procesos hijos
- [x] Implementar carga de configuración desde config.yaml sección `mcp.servers`
- [x] Implementar carga de servidores al startup del MCPHub
- [x] Implementar tracking de skills creados desde MCP (para cleanup)
- [x] Implementar cleanup de skills al desconectar servidor
- [x] Implementar detección de ciclo MCP (depth header)
- [x] Registrar MCPHub en el lifecycle de la aplicación (startup/shutdown)

## Frontend

- [x] (No aplica)

## Tests

- [x] Test unitario: MCPHub.Register() agrega servidor al mapa
- [x] Test unitario: MCPHub.Unregister() remueve servidor y cierra conexión
- [x] Test unitario: MCPHub.Servers() devuelve lista de servidores
- [x] Test unitario: MCPClient.Initialize() envía initialize y recibe respuesta
- [x] Test unitario: MCPClient.ListTools() parsea tools desde JSON-RPC
- [x] Test unitario: MCPClient.CallTool() envía args y recibe resultado
- [x] Test unitario: MCPClient.CallTool() con error devuelve error MCP
- [x] Test unitario: MCPClient.Close() termina proceso hijo
- [x] Test unitario: SkillAdapter.Sync() crea skills en SkillService
- [x] Test unitario: SkillAdapter.Sync() actualiza skills existentes
- [x] Test unitario: SkillAdapter.Execute() llama MCPClient.CallTool()
- [x] Test unitario: SkillAdapter.Execute() traduce error MCP a error Skill
- [x] Test unitario: discovery periódico refresca tools
- [x] Test unitario: reconnect después de disconnect
- [x] Test unitario: shutdown mata todos los procesos hijos
- [x] Test integración: spawn MCP server mock, conectar, descubrir, ejecutar tool
- [x] Test integración: kill MCP mock → detectar disconnect → reconnect
- [x] Test integración: servidor MCP mock inestable (crash loop) → circuit breaker
- [x] Sabotaje: shutdown sin matar hijos → procesos zombi
- [x] Sabotaje: no limpiar skills al disconnect → skills huerfanos
- [x] Sabotaje: no detectar stdin EOF → servidor muerto no detectado

## Cierre

- [x] Verificación manual: configurar servidor MCP externo en config.yaml, iniciar Domain, verificar skills creados
- [x] Verificación manual: ejecutar skill derivado de MCP externo
- [x] Verificación manual: matar proceso MCP externo, verificar reconnect
- [x] Suite verde: `go test ./internal/mcp/hub/... ./internal/mcp/client/...`
- [x] Documentar configuración de MCP externos en docs/mcp-servers.md
