# Proposal: issue-26.4-graceful-shutdown

## Intención

SIGTERM handler que drena gracefully en orden correcto: readiness off → grace ELB → HTTP shutdown → workers ctx cancel → pool close. Total <30s.

## Scope

- Manager `shutdown.Coordinator` orchestra
- Readiness probe state machine (ready → draining)
- HTTP `Shutdown(ctx)` con timeout
- Workers context propagation
- Pool close graceful
- Checkpoint forzado en workers para flows durables (issue-09.6 compat)
- Métricas
- Tests con `os.Signal` simulation

## Enfoque

1. Subscribe signal.Notify SIGTERM/SIGINT
2. Coordinator pipeline con timeouts por etapa
3. context.Background → context.WithCancel para todo subsistema

## Riesgos

- Workers que ignoran ctx: linter detecta `for {}` sin select ctx.Done()
- Pool close mid-tx: drain logic per pool

## Testing

- Signal simulado → orden correcto
- HTTP in-flight termina antes que pool close
- Worker mid-step → checkpoint forzado
- Timeout 30s → forced kill loggeado
