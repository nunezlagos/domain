# Tasks: issue-09.3-flow-state-machine

## Backend

- [ ] Crear modelo `FlowRun` y `StepRunState` en `internal/models/domain_flow_run.go`
- [ ] Implementar `FlowStateMachine` struct con mapa de transiciones válidas
- [ ] Implementar método `Transition(event)` que valida y ejecuta transición
- [ ] Implementar método `AllowedTransitions()` para consulta
- [ ] Crear migración SQL para tabla `flow_runs` con JSONB step states
- [ ] Implementar `FlowRunRepository` con Create, GetByID, UpdateStatus
- [ ] Implementar `FlowRunner` — orquesta steps según DAG
- [ ] Implementar lógica de detección de próximos steps (dependencias satisfechas)
- [ ] Implementar `FlowContext` compartido entre steps (map[string]StepResult)
- [ ] Implementar pause/resume con context cancel + snapshot
- [ ] Implementar cancel con propagación a todos los steps activos
- [ ] Crear handler REST: POST /flows/:slug/run
- [ ] Crear handler REST: GET /flow-runs/:id
- [ ] Crear handler REST: POST /flow-runs/:id/pause
- [ ] Crear handler REST: POST /flow-runs/:id/resume
- [ ] Crear handler REST: POST /flow-runs/:id/cancel
- [ ] Implementar SSE endpoint GET /flow-runs/:id/stream

## Tests

- [ ] Test unitario: todas las transiciones válidas de flow state machine
- [ ] Test unitario: todas las transiciones inválidas (completed→running, etc.)
- [ ] Test unitario: todas las transiciones válidas de step state machine
- [ ] Test unitario: DAG traversal con steps lineales
- [ ] Test unitario: DAG traversal con parallel branches
- [ ] Test unitario: DAG traversal con step fallido detiene ejecución
- [ ] Test unitario: FlowContext pasa resultados correctamente
- [ ] Test unitario: FlowContext step fallido no disponible
- [ ] Test de integración: ejecución linear completa
- [ ] Test de integración: ejecución con parallel (diamante)
- [ ] Test de integración: pause → resume cycle
- [ ] Test de integración: cancel en medio de step
- [ ] Test de integración: consulta GET flow-run después de completar
- [ ] Sabotaje: remover validación de transición → test falla

## Cierre

- [ ] Verificación manual: crear flow, ejecutar, ver estado via GET
- [ ] Verificación manual: pause → resume en flow con wait step
- [ ] Suite verde
