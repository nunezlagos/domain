# Design: HU-11.2-selfhosted-runner

## Decisión arquitectónica

```
┌─────────────────────────────────────────────────────────┐
│                    Servidor Domain                      │
│  ┌──────────────────────────────────────────────────┐   │
│  │           RunnerService (server-side)             │   │
│  │  - RegisterAgent()                                │   │
│  │  - AssignTask()                                   │   │
│  │  - ReceiveResult()                                │   │
│  │  - HeartbeatWatcher()                             │   │
│  └──────────────────────┬───────────────────────────┘   │
│                         │ WS / gRPC                     │
└─────────────────────────┼───────────────────────────────┘
                          │
┌─────────────────────────┼───────────────────────────────┐
│              domain-runner (Go binary)                  │
│  ┌──────────────────────┴───────────────────────────┐   │
│  │               Agent (agent.go)                    │   │
│  │  ┌──────────┐  ┌───────────┐  ┌──────────────┐  │   │
│  │  │Connection│  │ Executor  │  │ Heartbeater  │  │   │
│  │  │ - ws()   │  │ - local() │  │ - ping/pong  │  │   │
│  │  │ - grpc() │  │ - docker()│  │              │  │   │
│  │  │ - recon  │  └───────────┘  └──────────────┘  │   │
│  │  │   nect   │                                    │   │
│  │  └──────────┘                                    │   │
│  └──────────────────────────────────────────────────┘   │
│  Runtimes: python3, node, go, sh                        │
└─────────────────────────────────────────────────────────┘
```

**Decisión:** WebSocket como transporte principal (menor latencia, full-duplex, amplio soporte). gRPC como fallback para entornos que lo requieran. El runner es stateless: no persiste estado local, todo se sincroniza vía servidor.

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|---|---|
| SSH-based runner | Mayor latencia, no full-duplex, complejidad de key management |
| Long-polling HTTP | Mayor latencia, overhead de HTTP por mensaje, no streaming |
| NATS/Message Queue | Dependencia externa adicional, overkill para este caso de uso |
| Rust binary | Go da mejor experiencia de distribución (single binary cross-compile) |
| Python runner | Mayor consumo de memoria, más dependencias, menos performance |

## Diagrama

```
─── FLUJO DE REGISTRO ───

Runner                    Servidor
  │                         │
  │──── WS CONNECT ────────►│
  │                         │
  │──── register ──────────►│
  │     {token, hostname,   │
  │      capabilities}      │
  │                         │── validate JWT
  │                         │── save to DB
  │◄──── registered ────────│
  │     {runner_id,         │
  │      heartbeat_interval}│
  │                         │

─── FLUJO DE EJECUCIÓN ───

Runner                    Servidor
  │                         │
  │◄──── execute ───────────│
  │     {task_id, type,     │
  │      payload, timeout}  │
  │                         │
  │── lock task_id          │
  │── exec (local/docker)   │
  │── capture output        │
  │                         │
  │──── result ────────────►│
  │     {task_id, status,   │
  │      stdout, stderr,    │
  │      exit_code,         │
  │      duration}          │
  │                         │
  │── unlock → idle         │
```

## TDD plan

1. **Red:** Test que Agent register envía token al servidor
2. **Green:** Implementar `Agent.Register()` con Connection.Write
3. **Refactor:** Extraer Connection interface
4. **Red:** Test que executor local corre `echo hi` y captura stdout
5. **Green:** Implementar `LocalExecutor.Execute()` con os/exec
6. **Red:** Test que timeout cancela ejecución local
7. **Green:** Implementar context.WithTimeout en LocalExecutor
8. **Red:** Test que reconnect envía runner_id y token
9. **Green:** Implementar backoff exponencial en Connection
10. **Red:** Test que heartbeat mantiene conexión activa
11. **Green:** Implementar Heartbeater goroutine
12. **Sabotaje:** No enviar heartbeat → runner debe marcarse offline
13. **Integration:** WS mock server + runner real

## Riesgos y mitigación

- **Código hostil en host:** `sandbox: none` es inseguro por diseño. Mitigación: warning en startup, sandbox docker por defecto.
- **Token leak:** El token se pasa como flag CLI. Mitigación: soportar `--token-file` para leer desde archivo con permisos 600.
- **Reconnect storm:** Muchos runners reconectando simultáneamente. Mitigación: jitter en backoff.
- **Binary size:** Cross-compile Go con `-ldflags="-s -w"` y UPX compression.
- **Update distribution:** El runner debería auto-actualizarse. Mitigación: futuro, por ahora update manual.
