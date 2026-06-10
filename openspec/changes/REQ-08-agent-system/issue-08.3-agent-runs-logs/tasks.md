# Tasks: issue-08.3-agent-runs-logs

## Backend

- [ ] Crear migración SQL para tablas `agent_runs` y `run_logs`
- [ ] Implementar `RunRepository` con métodos: CreateRun, UpdateRun, AppendLog, GetRun, ListRuns, GetLogs
- [ ] Implementar `RunLogger` service con métodos: LogRunCreated, LogLLMCall, LogSkillExecution, LogRunCompleted, LogRunFailed
- [ ] Implementar cálculo de costo al finalizar run: tokens_input * cost_input + tokens_output * cost_output
- [ ] Implementar endpoints: GET /agents/:id/runs (con filtros y paginación)
- [ ] Implementar endpoint: GET /runs/:run_id
- [ ] Implementar endpoint: GET /runs/:run_id/log (con paginación por secuencia)
- [ ] Integrar `RunLogger` con `AgentExecutor` (issue-08.2)
- [ ] Agregar límite de tamaño en prompt/response (10KB) con truncamiento
- [ ] Implementar política de retención: purge de runs > 30 días (configurable)

## Tests

- [ ] Test unitario: RunLogger registra eventos en orden
- [ ] Test unitario: cálculo de costo con snapshot de precios
- [ ] Test unitario: filtros de status en ListRuns
- [ ] Test unitario: paginación (limit/offset)
- [ ] Test unitario: run sin logs retorna array vacío
- [ ] Test de integración: CRUD runs + logs en DB real
- [ ] Test de integración: logging durante ejecución de agente real
- [ ] Test E2E: escenarios Gherkin del hu.md vía API
- [ ] Sabotaje: run sin logs → array vacío, no error

## Cierre

- [ ] Verificación manual: ejecutar agente, consultar logs
- [ ] Suite verde completa
- [ ] Documentar endpoints y estructura de logs
