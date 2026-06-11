# Tasks: issue-09.3-flow-state-machine

## Backend

- [x] Crear modelo `FlowRun` y `StepRunState` en `internal/service/flow/state_machine.go` (FlowRunModel, StepRunState)
- [x] Implementar `FlowStateMachine` struct con mapa de transiciones válidas
- [x] Implementar método `AllowedTransitions()` para consulta
- [x] Implementar método `ValidateTransition()` que valida transiciones
- [x] Crear migración SQL para tabla `flow_runs` con JSONB step states (ya existía)
- [x] Implementar `FlowRunRepository` con Create, GetByID, UpdateStatus (ya existía en runner/flow)
- [x] Implementar `FlowRunner` — orquesta steps según DAG (ya existía)
- [x] Implementar lógica de detección de próximos steps (dependencias satisfechas) (ya existía)
- [x] Implementar `FlowContext` compartido entre steps (map[string]StepResult) (ya existía como stepOutputs)
- [x] Implementar pause/resume con context cancel + snapshot (service.PauseRun/ResumeRun + runner context tracking)
- [x] Implementar cancel con propagación a todos los steps activos (service.CancelRun + runner.CancelRun)
- [ ] Crear handler REST: POST /flows/:slug/run (ya existía como POST /flows/{id}/run)
- [ ] Crear handler REST: GET /flow-runs/:id
- [ ] Crear handler REST: POST /flow-runs/:id/pause
- [ ] Crear handler REST: POST /flow-runs/:id/resume
- [ ] Crear handler REST: POST /flow-runs/:id/cancel
- [ ] Implementar SSE endpoint GET /flow-runs/:id/stream

## Tests

- [x] Test unitario: todas las transiciones válidas de flow state machine
- [x] Test unitario: todas las transiciones inválidas (completed→running, etc.)
- [x] Test unitario: todas las transiciones válidas de step state machine
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
- [x] Sabotaje: remover validación de transición → test falla

## Cierre

- [ ] Verificación manual: crear flow, ejecutar, ver estado via GET
- [ ] Verificación manual: pause → resume en flow con wait step
- [ ] Suite verde
