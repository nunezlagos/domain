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

- [ ] Implementar `MCPHub` struct en `internal/mcp/hub/hub.go` con mapa de servidores
- [ ] Implementar `ExternalServer` struct: name, transport, command, args, env, status, tools, conn
- [ ] Implementar `MCPClient` en `internal/mcp/client/client.go`: initialize, listTools, callTool, close
- [ ] Implementar conexión stdio con `exec.CommandContext`: spawn proceso, pipes stdin/stdout
- [ ] Implementar JSON-RPC message layer sobre pipes (readline/writeline)
- [ ] Implementar `MCPClient.Initialize()` con handshake protocol
- [ ] Implementar `MCPClient.ListTools()` para discovery
- [ ] Implementar `MCPClient.CallTool(name, args)` para ejecución
- [ ] Implementar `MCPClient.Close()` con graceful shutdown + SIGKILL fallback
- [ ] Implementar `SkillAdapter` en `internal/mcp/hub/skill_adapter.go`: bridge SkillExecutor → MCPClient
- [ ] Implementar `SyncWithSkillService()` que crea/actualiza skills por cada tool descubierta
- [ ] Implementar discovery periódico con ticker (configurable por servidor)
- [ ] Implementar reconnect con backoff exponencial tras pérdida de conexión
- [ ] Implementar detección de desconexión (stdin EOF, process exit)
- [ ] Implementar circuit breaker: after N failures, mark as failed, stop reconnect
- [ ] Implementar `MCPHub.Shutdown()` que mata todos los procesos hijos
- [ ] Implementar carga de configuración desde config.yaml sección `mcp.servers`
- [ ] Implementar carga de servidores al startup del MCPHub
- [ ] Implementar tracking de skills creados desde MCP (para cleanup)
- [ ] Implementar cleanup de skills al desconectar servidor
- [ ] Implementar detección de ciclo MCP (depth header)
- [ ] Registrar MCPHub en el lifecycle de la aplicación (startup/shutdown)

## Frontend

- [ ] (No aplica)

## Tests

- [ ] Test unitario: MCPHub.Register() agrega servidor al mapa
- [ ] Test unitario: MCPHub.Unregister() remueve servidor y cierra conexión
- [ ] Test unitario: MCPHub.Servers() devuelve lista de servidores
- [ ] Test unitario: MCPClient.Initialize() envía initialize y recibe respuesta
- [ ] Test unitario: MCPClient.ListTools() parsea tools desde JSON-RPC
- [ ] Test unitario: MCPClient.CallTool() envía args y recibe resultado
- [ ] Test unitario: MCPClient.CallTool() con error devuelve error MCP
- [ ] Test unitario: MCPClient.Close() termina proceso hijo
- [ ] Test unitario: SkillAdapter.Sync() crea skills en SkillService
- [ ] Test unitario: SkillAdapter.Sync() actualiza skills existentes
- [ ] Test unitario: SkillAdapter.Execute() llama MCPClient.CallTool()
- [ ] Test unitario: SkillAdapter.Execute() traduce error MCP a error Skill
- [ ] Test unitario: discovery periódico refresca tools
- [ ] Test unitario: reconnect después de disconnect
- [ ] Test unitario: shutdown mata todos los procesos hijos
- [ ] Test integración: spawn MCP server mock, conectar, descubrir, ejecutar tool
- [ ] Test integración: kill MCP mock → detectar disconnect → reconnect
- [ ] Test integración: servidor MCP mock inestable (crash loop) → circuit breaker
- [ ] Sabotaje: shutdown sin matar hijos → procesos zombi
- [ ] Sabotaje: no limpiar skills al disconnect → skills huerfanos
- [ ] Sabotaje: no detectar stdin EOF → servidor muerto no detectado

## Cierre

- [ ] Verificación manual: configurar servidor MCP externo en config.yaml, iniciar Domain, verificar skills creados
- [ ] Verificación manual: ejecutar skill derivado de MCP externo
- [ ] Verificación manual: matar proceso MCP externo, verificar reconnect
- [ ] Suite verde: `go test ./internal/mcp/hub/... ./internal/mcp/client/...`
- [ ] Documentar configuración de MCP externos en docs/mcp-servers.md
