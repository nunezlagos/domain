# Tasks: HU-01.3-health-version

## Backend

- [ ] Crear `internal/version/version.go` con variables Version, Commit, BuildTime
- [ ] Implementar handler `GET /health` en `internal/api/health.go`
- [ ] Integrar DB ping con timeout en health handler
- [ ] Registrar ruta `/health` en el router principal
- [ ] Guardar startTime al inicio de `main()` para uptime
- [ ] Agregar target `build` en Makefile con ldflags
- [ ] Agregar target `run` que compile con version y ejecute
- [ ] Implementar comando `domain version` en CLI
- [ ] Health response JSON con todos los campos

## Tests

- [ ] Test unitario: handler devuelve 200 con DB mock
- [ ] Test unitario: handler devuelve 503 con DB caída
- [ ] Test unitario: version default "dev"
- [ ] Test unitario: version se sobreescribe
- [ ] Test unitario: uptime positivo
- [ ] Test unitario: CLI `version` imprime correctamente
- [ ] Sabotaje: remover ldflags → confirmar "dev" → restaurar
- [ ] Sabotaje: DB mock siempre error → confirmar 503 → restaurar

## Cierre

- [ ] Verificación manual: `curl localhost:3000/health`
- [ ] Verificación manual: `domain version`
- [ ] Suite verde
