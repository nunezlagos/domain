# Tasks: HU-11.2-selfhosted-runner

## Backend

- [ ] Crear `cmd/domain-runner/main.go` con flags CLI (--server, --token, --work-dir, --sandbox, --max-concurrency, --log-level)
- [ ] Implementar `internal/runner/agent/agent.go`: ciclo de vida register → idle → execute → result → idle
- [ ] Implementar `internal/runner/connection/ws.go`: WebSocket client con reconnect exponencial
- [ ] Implementar protocolo de mensajes JSON sobre WS (register, execute, result, heartbeat, error)
- [ ] Implementar `internal/runner/executor/local.go`: ejecución local con os/exec, captura stdout/stderr
- [ ] Implementar `internal/runner/executor/docker.go`: wrapper sobre HU-11.1 SandboxManager
- [ ] Implementar timeout enforcement con context.WithTimeout
- [ ] Implementar Heartbeater: goroutine con ping cada 30s, timeout de 90s
- [ ] Implementar capabilities: detección de runtimes instalados (python3 --version, node --version, etc.)
- [ ] Implementar graceful shutdown (SIGTERM/SIGINT): terminar tarea actual, enviar offline, cerrar WS
- [ ] Implementar token auth: leer token de flag o archivo, enviar en register
- [ ] Implementar logging local a archivo rotativo (logrus o zap)
- [ ] Server-side: `RunnerService.RegisterAgent()` con validación de JWT
- [ ] Server-side: `RunnerService.AssignTask()` con round-robin básico
- [ ] Server-side: `RunnerService.ReceiveResult()` con actualización de estado
- [ ] Server-side: `RunnerService.HeartbeatWatcher()` goroutine que marca offline
- [ ] Server-side: endpoint WebSocket `/ws/runners` para conexión de runners
- [ ] Server-side: modelo `Runner` en DB: id, org_id, token, hostname, status, capabilities, last_heartbeat
- [ ] Server-side: API REST para CRUD de tokens de runner (generar, listar, revocar)
- [ ] CI: cross-compile para linux/amd64, linux/arm64, darwin/amd64, darwin/arm64

## Frontend

- [ ] (No aplica - el runner es CLI binary)

## Tests

- [ ] Test unitario: Agent.Register() envía mensaje register correcto
- [ ] Test unitario: Agent recibe execute y ejecuta con executor mock
- [ ] Test unitario: Agent envía result correcto después de ejecución
- [ ] Test unitario: timeout cancela ejecución y envía status timeout
- [ ] Test unitario: LocalExecutor ejecuta comando y captura stdout/stderr
- [ ] Test unitario: LocalExecutor timeout con sleep prolongado
- [ ] Test unitario: Connection reconnect con backoff exponencial
- [ ] Test unitario: Connection reconecta y reenvía runner_id + token
- [ ] Test unitario: Heartbeater envía ping cada intervalo
- [ ] Test unitario: graceful shutdown envía offline
- [ ] Test unitario: server-side RunnerService valida token JWT
- [ ] Test unitario: server-side HeartbeatWatcher marca offline después de timeout
- [ ] Test integración: WS mock server + Agent real
- [ ] Test integración: ejecución real de python3 con LocalExecutor
- [ ] Test integración: reconexión después de caída de red simulada (iptables)
- [ ] Sabotaje: no enviar heartbeat → runner debe quedar offline
- [ ] Sabotaje: no hacer cleanup de tarea al reconectar → tarea queda colgada
- [ ] Sabotaje: no capturar stderr → error se pierde

## Cierre

- [ ] Verificación manual: registrar runner desde CLI
- [ ] Verificación manual: ejecutar code_exec step que se asigna al runner
- [ ] Verificación manual: desconectar runner y verificar offline detection
- [ ] Verificación manual: reconexión automática
- [ ] Suite verde: `go test ./internal/runner/... ./cmd/domain-runner/...`
- [ ] Documentar despliegue de runner en docs/selfhosted-runner.md
