# Tasks: HU-10.1-cron-schedules

## Backend

- [ ] Crear modelo `Cron` en `internal/models/cron.go`
- [ ] Crear migración SQL para tabla `crons` con constraints y check
- [ ] Crear migración SQL para tabla `cron_executions`
- [ ] Implementar `CronRepository` con Create, GetByID, GetByProjectID, Update, Delete
- [ ] Implementar método `GetDueCrons(now)` con FOR UPDATE SKIP LOCKED
- [ ] Implementar validación de expresión cron (robfig/cron)
- [ ] Implementar validación de timezone (IANA)
- [ ] Implementar cálculo de next_run con timezone
- [ ] Implementar scheduler worker (goroutine con ticker 60s)
- [ ] Implementar ejecución de flow desde scheduler
- [ ] Implementar ejecución de agente desde scheduler
- [ ] Implementar detección de overlap (skip si previous sigue running)
- [ ] Implementar historial de ejecuciones (insert en cron_executions)
- [ ] Crear handler REST: CRUD /api/v1/crons
- [ ] Crear handler REST: GET /api/v1/crons/:id/history
- [ ] Crear handler REST: PATCH /api/v1/crons/:id (enable/disable)

## Tests

- [ ] Test unitario: parseo cron expression válida
- [ ] Test unitario: parseo cron expression inválida
- [ ] Test unitario: cálculo next_run UTC
- [ ] Test unitario: cálculo next_run con timezone
- [ ] Test unitario: validación timezone inválida
- [ ] Test unitario: validación flow_slug XOR agent_slug
- [ ] Test unitario: CRUD repository
- [ ] Test de integración: scheduler ejecuta cron debido
- [ ] Test de integración: scheduler salta cron deshabilitado
- [ ] Test de integración: scheduler evita doble ejecución
- [ ] Test de integración: historial se registra correctamente
- [ ] Test de integración: overlap skip
- [ ] Sabotaje: sacar FOR UPDATE → test de doble ejecución falla

## Cierre

- [ ] Verificación manual: crear cron con expresión cada 5 min, esperar ejecución
- [ ] Verificación manual: deshabilitar cron, confirmar que no ejecuta
- [ ] Suite verde
