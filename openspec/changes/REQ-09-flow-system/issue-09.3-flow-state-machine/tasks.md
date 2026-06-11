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
- [x] Crear handler REST: POST /flows/{id}/run (por id en lugar de slug — UUIDs en path per api.md)
- [x] Crear handler REST: GET /flow-runs/:id → handler/flowrun.go getFlowRun (run + steps con progreso) — 2026-06-10
- [x] Crear handler REST: POST /flow-runs/:id/pause → pauseFlowRun (409 invalid transition) — 2026-06-10
- [x] Crear handler REST: POST /flow-runs/:id/resume → resumeFlowRun — 2026-06-10
- [x] Crear handler REST: POST /flow-runs/:id/cancel → cancelFlowRun (+ propagación best-effort a runner local) — 2026-06-10
- [x] Implementar SSE endpoint GET /flow-runs/:id/stream → streamFlowRun (LISTEN flow_step_progress + snapshots status cada 5s, cierra en terminal; directiva response-shape-lint:allow) — 2026-06-10

## Tests

- [x] Test unitario: todas las transiciones válidas de flow state machine
- [x] Test unitario: todas las transiciones inválidas (completed→running, etc.)
- [x] Test unitario: todas las transiciones válidas de step state machine
- [x] Test unitario: DAG traversal con steps lineales → TestFlow_LinearTraversal_ContextAccumulates (orden + filas) — 2026-06-10
- [x] Test unitario: DAG traversal con parallel branches → TestFlow_ParallelDiamond — 2026-06-10
- [x] Test unitario: DAG traversal con step fallido detiene ejecución → TestFlow_OnErrorFailAborts
- [x] Test unitario: FlowContext pasa resultados correctamente → TestFlow_LinearTraversal_ContextAccumulates (outputs s1..s3) + TestResolveTemplate_StepOutputs
- [x] Test unitario: FlowContext step fallido no disponible → TestFlow_OnErrorContinue (error marker en lugar de output)
- [x] Test de integración: ejecución linear completa → TestFlow_BasicSkillRun + LinearTraversal
- [x] Test de integración: ejecución con parallel (diamante) → TestFlow_ParallelDiamond — 2026-06-10
- [x] Test de integración: pause → resume cycle → TestFlowRunAPI_PauseResumeCancel (API end-to-end) — 2026-06-10
- [x] Test de integración: cancel en medio de step → TestFlow_CancelMidStep — 2026-06-10
- [x] Test de integración: consulta GET flow-run después de completar → TestFlowRunAPI_Lifecycle — 2026-06-10
- [x] Sabotaje: remover validación de transición → test falla

## Cierre

- [x] Verificación manual: crear flow, ejecutar, ver estado via GET → cubierto end-to-end por TestFlowRunAPI_Lifecycle (httptest con auth real)
- [x] Verificación manual: pause → resume en flow con wait step → TestFlowRunAPI_PauseResumeCancel + TestFlow_WaitSignal_DeliveredResumes
- [x] Suite verde → 2026-06-10: suite corta + integración flows/handler verdes
