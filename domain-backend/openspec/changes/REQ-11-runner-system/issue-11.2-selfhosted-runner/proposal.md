# Proposal: issue-11.2-selfhosted-runner

## Intención

Crear un agente runner Go independiente (`domain-runner`) que se conecta al servidor Domain mediante WebSocket/gRPC, recibe tareas de ejecución, las procesa localmente y reporta resultados. Permite a usuarios con restricciones de infraestructura ejecutar cargas de trabajo sin depender del sandbox cloud.

## Scope

**Incluye:**
- Binary independiente `domain-runner` (Go, single binary)
- Conexión WebSocket con el servidor (con fallback a gRPC)
- Protocolo de registro: token + metadatos + capabilities
- Recepción y ejecución de tareas code_exec y skill_exec
- Captura de stdout/stderr/exit_code/duration
- Timeout enforcement local
- Heartbeat cada 30s, offline detection a los 90s
- Reconexión con backoff exponencial (max 30s, max 5 min)
- Execution sandbox opcional (Docker vía issue-11.1, o directo)
- Capabilities negotiation: el runner declara qué runtimes soporta
- Token-based authentication con rotación
- Logging local a archivo rotativo
- Graceful shutdown (SIGTERM/SIGINT: terminar tarea actual, enviar offline)

**No incluye:**
- Dashboard de gestión de runners (se hará en REQ-16-web-ui)
- Asignación automática de tareas a runners (round-robin por ahora)
- Runners con GPU
- Ejecución de tareas de tipo container (solo code_exec y skill_exec)
- Windows support (solo Linux/Mac)

## Enfoque técnico

1. `cmd/domain-runner/main.go`: entry point, flags CLI (--server, --token, --work-dir, --sandbox)
2. `internal/runner/agent/agent.go`: `Agent` struct que maneja ciclo de vida
3. `internal/runner/connection/ws.go`: WebSocket client con reconnect
4. Protocolo: mensajes JSON sobre WebSocket con campos `type`, `payload`, `id`
5. `internal/runner/executor/local.go`: ejecución local directa (os/exec)
6. `internal/runner/executor/docker.go`: ejecución via issue-11.1 SandboxManager
7. Token: JWT firmado por el servidor, el runner lo envía en cada reconexión
8. Metadatos: hostname, OS, arch, version, capabilities
9. Heartbeat: goroutine que envía ping cada 30s, expect pong
10. Graceful shutdown: terminar tarea actual con status `interrupted`, enviar offline, cerrar WS

## Riesgos

- **Código malicioso en host del runner:** Si `sandbox: none`, el código corre sin aislamiento. Mitigación: documentar riesgo, recomendar sandbox docker, el runner corre con permisos mínimos.
- **Token comprometido:** Un token robado permite registrar runners maliciosos. Mitigación: rotación de tokens, revocación desde UI, expiración automática.
- **Red inestable:** Reconnect con backoff, buffer de tareas si está ejecutando, timeout de conexión.
- **Versiones de runtime:** El runner declara capabilities pero el código puede requerir versiones específicas. Mitigación: capabilities semver, error temprano si runtime no encontrado.
- **Consumo de recursos:** Múltiples tareas concurrentes pueden saturar el host. Mitigación: max_concurrency configurable, queue local.

## Testing

- Unit tests: Agent lifecycle (register, execute, heartbeat, reconnect)
- Unit tests: Executor mock
- Unit tests: Connection manager con reconnect
- Integration: WS server mock que envía tareas
- Integration: ejecución real de python/node en local executor
- Integration: timeout enforcement
- Integration: reconexión después de caída de red simulada
- E2E: runner real conectándose a servidor real
