# Tasks: issue-08.3-agent-runs-logs

## Backend

- [x] Crear migración SQL para tablas `agent_runs` y `run_logs`
- [x] Implementar `RunRepository` con métodos: CreateRun, UpdateRun, AppendLog, GetRun, ListRuns, GetLogs
- [x] Implementar `RunLogger` service con métodos: LogRunCreated, LogLLMCall, LogSkillExecution, LogRunCompleted, LogRunFailed
- [x] Implementar cálculo de costo al finalizar run: tokens_input * cost_input + tokens_output * cost_output
- [x] Implementar endpoints: GET /agents/:id/runs (con filtros y paginación)
- [x] Implementar endpoint: GET /runs/:run_id
- [x] Implementar endpoint: GET /runs/:run_id/log (con paginación por secuencia)
- [x] Integrar `RunLogger` con `AgentExecutor` (issue-08.2)
- [x] Agregar límite de tamaño en prompt/response (10KB) con truncamiento
- [x] Implementar política de retención: purge de runs > 30 días (configurable)

## Tests

- [x] Test unitario: RunLogger registra eventos en orden
- [x] Test unitario: cálculo de costo con snapshot de precios
- [x] Test unitario: filtros de status en ListRuns
- [x] Test unitario: paginación (limit/offset)
- [x] Test unitario: run sin logs retorna array vacío
- [x] Test de integración: CRUD runs + logs en DB real
- [x] Test de integración: logging durante ejecución de agente real
- [x] Test E2E: escenarios Gherkin del hu.md vía API
- [x] Sabotaje: run sin logs → array vacío, no error

## Cierre

- [x] Verificación manual: ejecutar agente, consultar logs
- [x] Suite verde completa
- [x] Documentar endpoints y estructura de logs
