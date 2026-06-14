# Proposal: issue-12.4-mcp-bidirectional

## Intención

Hacer que Domain sea un participante MCP bidireccional: actúa como servidor MCP (issue-12.1) proveyendo tools de la plataforma, Y como cliente MCP consumiendo servidores externos. Los tools de MCPs externos se convierten automáticamente en skills de Domain, accesibles desde flows, agentes y la CLI.

## Scope

**Incluye:**
- MCP Hub: registry de servidores MCP externos conectados
- MCP Client: implementación de cliente MCP que se conecta vía stdio
- Tool Discovery automático: `tools/list` periódico sobre servidores externos
- Skill Adapter: crea/actualiza skills en Domain por cada tool descubierta
- Skill execution bridge: cuando se ejecuta un skill originado en MCP externo, llamar `tools/call` al servidor correspondiente
- Ciclo de vida de conexiones: connect, heartbeat, reconnect, disconnect, failed
- Configuración de servidores MCP externos vía config.yaml
- API interna para registrar/desregistrar servidores MCP externos
- Manejo de errores: propagación de errores MCP como errores de skill
- Timeout por tool externa configurable

**No incluye:**
- Transporte HTTP/SSE para clientes MCP externos (solo stdio)
- Autenticación entre MCPs (asume trust por ahora)
- GUI para gestionar conexiones MCP (solo config file)
- MCP server con capacidades de resources o prompts (solo tools)

## Enfoque técnico

1. `internal/mcp/hub/hub.go`: `MCPHub` struct singleton que mantiene `map[string]*ExternalServer`
2. `ExternalServer` struct: name, transport, command, args, env, status, tools, conn
3. `internal/mcp/client/client.go`: `MCPClient` implementa el protocolo MCP del lado cliente
4. Client usa `mark3labs/mcp-go` como cliente (inicializa, lista tools, llama tools)
5. Conexión stdio: `exec.CommandContext` para spawn del proceso externo, stdin/stdout pipes
6. Tool Discovery: goroutine periódica que llama `tools/list` y sincroniza con `SkillService`
7. `internal/mcp/hub/skill_adapter.go`: bridge que implementa `SkillExecutor` interface y traduce a llamada MCP
8. Ejecución bridge: recibe `SkillExecutionRequest` → llama `tools/call` al servidor MCP externo → devuelve resultado
9. Config en `config.yaml` sección `mcp.servers`:
```yaml
mcp:
  servers:
    - name: github-mcp
      transport: stdio
      command: npx
      args: ["@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${GITHUB_TOKEN}"
      discovery_interval: 5m
      tool_timeout: 30s
```

## Riesgos

- **Comandos arbitrarios:** La config permite ejecutar cualquier comando como MCP externo. Mitigación: solo admin puede modificar config, validación de comandos permitidos.
- **Procesos zombi:** Si Domain crashea, los procesos MCP externos hijos quedan vivos. Mitigación:杀掉进程树 al hacer shutdown, registro de PIDs.
- **Ciclo infinito:** MCP externo que llama a Domain que llama al externo. Mitigación: detección de ciclos con request ID tracing, max depth.
- **Inestabilidad de servidores externos:** Servidores stdio pueden crashear. Mitigación: reconnect con backoff, circuit breaker después de N fallos.
- **Memory leak:** Muchos servidores externos con muchas tools. Mitigación: límite de servidores conectados, cleanup de tools huerfanas.

## Testing

- Unit: MCPHub register/deregister/servers list
- Unit: MCPClient initialize + listTools + callTool con mock stdin/stdout
- Unit: SkillAdapter crea skills desde tools MCP
- Unit: SkillAdapter ejecuta tool externa y traduce resultado
- Unit: discovery periódico sincroniza tools
- Integration: spawn proceso MCP mock, conectar, descubrir tools, ejecutar tool
- Integration: reconnect después de kill del proceso mock
- Integration: bidireccional: Domain MCP server + MCP externo ambos funcionando
