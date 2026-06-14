# Tasks: issue-11.2-selfhosted-runner

## Backend

- [x] Crear `cmd/domain-runner/main.go` con flags CLI (--server, --token, --work-dir, --sandbox, --max-concurrency, --log-level)
- [x] Implementar `internal/runner/agent/agent.go`: ciclo de vida register → idle → execute → result → idle
- [x] Implementar `internal/runner/connection/ws.go`: WebSocket client con reconnect exponencial
- [x] Implementar protocolo de mensajes JSON sobre WS (register, execute, result, heartbeat, error)
- [x] Implementar `internal/runner/executor/local.go`: ejecución local con os/exec, captura stdout/stderr
- [x] Implementar `internal/runner/executor/docker.go`: wrapper sobre issue-11.1 SandboxManager
- [x] Implementar timeout enforcement con context.WithTimeout
- [x] Implementar Heartbeater: goroutine con ping cada 30s, timeout de 90s
- [x] Implementar capabilities: detección de runtimes instalados (python3 --version, node --version, etc.)
- [x] Implementar graceful shutdown (SIGTERM/SIGINT): terminar tarea actual, enviar offline, cerrar WS
- [x] Implementar token auth: leer token de flag o archivo, enviar en register
- [x] Implementar logging local a archivo rotativo (logrus o zap)
- [x] Server-side: `RunnerService.RegisterAgent()` con validación de JWT
- [x] Server-side: `RunnerService.AssignTask()` con round-robin básico
- [x] Server-side: `RunnerService.ReceiveResult()` con actualización de estado
- [x] Server-side: `RunnerService.HeartbeatWatcher()` goroutine que marca offline
- [x] Server-side: endpoint WebSocket `/ws/runners` para conexión de runners
- [x] Server-side: modelo `Runner` en DB: id, org_id, token, hostname, status, capabilities, last_heartbeat
- [x] Server-side: API REST para CRUD de tokens de runner (generar, listar, revocar)
- [x] CI: cross-compile para linux/amd64, linux/arm64, darwin/amd64, darwin/arm64

## Frontend

- [x] (No aplica - el runner es CLI binary)

## Tests

- [x] Test unitario: Agent.Register() envía mensaje register correcto
- [x] Test unitario: Agent recibe execute y ejecuta con executor mock
- [x] Test unitario: Agent envía result correcto después de ejecución
- [x] Test unitario: timeout cancela ejecución y envía status timeout
- [x] Test unitario: LocalExecutor ejecuta comando y captura stdout/stderr
- [x] Test unitario: LocalExecutor timeout con sleep prolongado
- [x] Test unitario: Connection reconnect con backoff exponencial
- [x] Test unitario: Connection reconecta y reenvía runner_id + token
- [x] Test unitario: Heartbeater envía ping cada intervalo
- [x] Test unitario: graceful shutdown envía offline
- [x] Test unitario: server-side RunnerService valida token JWT
- [x] Test unitario: server-side HeartbeatWatcher marca offline después de timeout
- [x] Test integración: WS mock server + Agent real
- [x] Test integración: ejecución real de python3 con LocalExecutor
- [x] Test integración: reconexión después de caída de red simulada (iptables)
- [x] Sabotaje: no enviar heartbeat → runner debe quedar offline
- [x] Sabotaje: no hacer cleanup de tarea al reconectar → tarea queda colgada
- [x] Sabotaje: no capturar stderr → error se pierde

## Cierre

- [x] Verificación manual: registrar runner desde CLI
- [x] Verificación manual: ejecutar code_exec step que se asigna al runner
- [x] Verificación manual: desconectar runner y verificar offline detection
- [x] Verificación manual: reconexión automática
- [x] Suite verde: `go test ./internal/runner/... ./cmd/domain-runner/...`
- [x] Documentar despliegue de runner en docs/selfhosted-runner.md
