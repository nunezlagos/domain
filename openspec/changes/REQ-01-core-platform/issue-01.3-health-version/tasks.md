# Tasks: issue-01.3-health-version

## Backend

- [x] Crear `internal/version/version.go` con variables Version, Commit, BuildTime
- [x] Implementar handler `GET /health` en `internal/api/health.go`
- [x] Integrar DB ping con timeout en health handler
- [x] Registrar ruta `/health` en el router principal
- [x] Guardar startTime al inicio de `main()` para uptime
- [x] Agregar target `build` en Makefile con ldflags
- [x] Agregar target `run` que compile con version y ejecute
- [x] Implementar comando `domain version` en CLI
- [x] Health response JSON con todos los campos

## Tests

- [x] Test unitario: handler devuelve 200 con DB mock
- [x] Test unitario: handler devuelve 503 con DB caída
- [x] Test unitario: version default "dev"
- [x] Test unitario: version se sobreescribe
- [x] Test unitario: uptime positivo
- [x] Test unitario: CLI `version` imprime correctamente
- [x] Sabotaje: remover ldflags → confirmar "dev" → restaurar
- [x] Sabotaje: DB mock siempre error → confirmar 503 → restaurar

## Cierre

- [x] Verificación manual: `curl localhost:3000/health`
- [x] Verificación manual: `domain version`
- [x] Suite verde
