# Design: HU-12.4-mcp-bidirectional

## Decisión arquitectónica

```
                      ┌────────────────────────────────┐
                      │         Domain Platform        │
                      │                                  │
                      │  ┌──────────────────────────┐   │
                      │  │     MCPServer (HU-12.1)   │   │
                      │  │  Provee tools de Domain  │   │
                      │  │  a clientes MCP externos  │   │
                      │  └──────────────────────────┘   │
                      │                                  │
                      │  ┌──────────────────────────┐   │
                      │  │        MCP Hub            │   │
                      │  │                           │   │
                      │  │  ┌─────────────────────┐ │   │
                      │  │  │  ExternalServer      │ │   │
                      │  │  │  - name: "github"    │ │   │
                      │  │  │  - status: connected │ │   │
                      │  │  │  - tools: [...]      │ │   │
                      │  │  │  - client: MCPClient │ │   │
                      │  │  └─────────────────────┘ │   │
                      │  │                           │   │
                      │  │  ┌─────────────────────┐ │   │
                      │  │  │  SkillAdapter        │ │   │
                      │  │  │  (bridge MCP→Skill)  │ │   │
                      │  │  └─────────────────────┘ │   │
                      │  └──────────────────────────┘   │
                      │                                  │
                      │  ┌──────────────────────────┐   │
                      │  │      SkillService         │   │
                      │  │  Skills nativos + MCP     │   │
                      │  └──────────────────────────┘   │
                      └──────────────────────────────────┘
                               │                 ▲
                               │ stdio           │ tools/call
                               ▼                 │
                      ┌──────────────────────────────────┐
                      │       MCP Server Externo          │
                      │  (github-mcp, db-mcp, etc)        │
                      │  Proceso hijo (exec.Command)       │
                      └──────────────────────────────────┘
```

**Decisión:** MCP Hub como orquestador central de conexiones salientes. Cada servidor externo es un proceso hijo con pipes stdin/stdout. SkillAdapter implementa `SkillExecutor` interface para que los skills derivados de MCP sean indistinguibles de skills nativos desde la perspectiva del resto del sistema.

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|---|---|
| Plugin system (hashicorp/go-plugin) | MCP ya es el estándar de plugin, no necesitamos otro |
| NATS/Kafka para MCP bridge | Overkill, MCP ya define protocolo request/response simple |
| Cada MCP externo en su propio goroutine sin hub | Complejidad de gestión, difícil monitorear estado global |
| Traducir MCP tools a funciones nativas en vez de skills | Skills ya son la abstracción correcta; MCP tools son skills externalizados |
| Soporte HTTP/SSE para externos | stdio es más simple y seguro para procesos locales |

## Diagrama

```
─── FLUJO DE CONEXIÓN Y DISCOVERY ───

MCPHub                     MCPClient                 Proceso Externo
  │                           │                           │
  │── Connect(name) ─────────►│                           │
  │                           │── exec.Command() ────────►│
  │                           │     (spawn proceso)       │
  │                           │◄──── stdout/stdin ────────│
  │                           │                           │
  │                           │── initialize ────────────►│
  │                           │◄── protocol_version ..... │
  │                           │                           │
  │                           │── tools/list ────────────►│
  │                           │◄── [{name, desc, schema}] │
  │                           │                           │
  │◄── tools discovered ─────│                           │
  │                           │                           │
  │── SyncWithSkillService ──►│                           │
  │     (crea/actualiza       │                           │
  │      skills por cada tool)│                           │
  │                           │                           │

─── FLUJO DE EJECUCIÓN DE SKILL MCP ───

SkillService              SkillAdapter              MCPClient          Proceso Ext.
  │                           │                        │                   │
  │── Execute(skill_id,       │                        │                   │
  │    params) ──────────────►│                        │                   │
  │                           │── find server + tool   │                   │
  │                           │                        │                   │
  │                           │── tools/call ─────────►│                   │
  │                           │    {name, args}        │── JSON-RPC ─────►│
  │                           │                        │◄── result ───────│
  │                           │◄── result ────────────│                   │
  │◄── result ───────────────│                        │                   │
  │                           │                        │                   │

─── FLUJO DE RECONEXIÓN ───

MCPHub                     MCPClient                 Proceso Externo
  │                           │                           │
  │                           │◄──── process dies ────────│
  │◄── disconnected ─────────│                           │
  │                           │                           │
  │── schedule reconnect ───►│                           │
  │     (backoff 1s,2s,4s)   │                           │
  │                           │── exec.Command() ────────►│
  │                           │     (re-spawn)            │
  │                           │◄──── connected ──────────│
  │                           │── initialize + tools/list │
  │◄── reconnected ──────────│                           │
```

## TDD plan

1. **Red:** Test que MCPHub registra un ExternalServer correctamente
2. **Green:** Implementar MCPHub.Register() + mapa de servidores
3. **Red:** Test que MCPClient.Connect() spawns proceso y hace initialize
4. **Green:** Implementar MCPClient con exec.CommandContext + initialize
5. **Red:** Test que MCPClient.DiscoverTools() devuelve tools del servidor
6. **Green:** Implementar tools/list vía JSON-RPC
7. **Red:** Test que SkillAdapter crea skills desde tools descubiertas
8. **Green:** Implementar SyncWithSkillService()
9. **Red:** Test que ejecución de skill MCP llama tools/call y devuelve resultado
10. **Green:** Implementar SkillAdapter.Execute() con MCPClient.CallTool()
11. **Red:** Test que reconnect después de proceso muerto funciona
12. **Green:** Implementar reconnect con backoff
13. **Red:** Test que discovery periódico refresca tools
14. **Green:** Implementar DiscoveryLoop con ticker
15. **Red:** Test que shutdown mata procesos hijos
16. **Green:** Implementar Shutdown() con process kill
17. **Sabotaje:** No limpiar skills cuando MCP server se desconecta → skills huerfanos
18. **Sabotaje:** No matar procesos hijos en shutdown → procesos zombi

## Riesgos y mitigación

- **Procesos zombi:** Señal SIGKILL a proceso hijo, esperar Wait(), log si no termina. Mitigación: proceso monitor con timeout de kill.
- **Comandos maliciosos en config:** Solo admin puede editar config. Mitigación: validar command contra whitelist, warning si es desconocido.
- **Ciclo MCP infinito:** Servidor MCP externo que a su vez llama a Domain. Mitigación: header `X-Domain-MCP-Depth` en initialize, max depth 3.
- **Memory leak por tools no usadas:** Skills MCP no se limpian si el server se va. Mitigación: cleanup on disconnect, TTL para skills no sincronizados.
- **Deadlock en exec.Command:** Si el proceso externo no escribe nada y llena buffer de stdout. Mitigación: usar pipes con buffer grande, timeout en lectura.
