# Proposal: issue-08.3-agent-runs-logs

## Intención

Implementar logging estructurado de runs de agente: tabla `agent_runs` con metadata del run, y tabla `run_logs` con detalle de cada LLM call y skill execution. Endpoints con filtros y paginación.

## Scope

**In scope:**
- Tabla `agent_runs`: id, agent_id, project_id, status, input, output, tokens_used, cost, started_at, ended_at, error
- Tabla `run_logs`: id, run_id, type (llm_call|skill_execution), timestamp, prompt, response, tool_calls, skill_name, args, result, duration, tokens, status
- Endpoints: GET /agents/:id/runs, GET /runs/:run_id, GET /runs/:run_id/log
- Filtros: status, date range, limit/offset pagination
- Cálculo de costo basado en tokens + modelo (desde model registry)
- Log streaming opcional (tail de logs en vivo)

**Out of scope:**
- Export de logs a CSV/JSON
- Alertas sobre runs fallidos (issue-15.3)
- Visualización web (REQ-16)

## Enfoque técnico

- `RunLogger` service que es llamado por `AgentExecutor` en cada etapa
- Etapas: RunCreated → LLMCall → SkillExecution → RunCompleted / RunFailed
- Cada etapa escribe a `run_logs` con el tipo correspondiente
- Costo se calcula al finalizar el run: `tokens_input * cost_input + tokens_output * cost_output` (desde model registry)
- Endpoints con repositorio con filtros SQL parametrizados

## Riesgos

- Volumen de logs puede ser alto (cada tool_call genera múltiples entradas) → paginación obligatoria
- Prompts/responses grandes almacenados en DB → considerar límite de tamaño en run_logs.prompt/response
- Costo de modelo puede cambiar → guardar cost input/output en el run al momento de creación (snapshot)

## Testing

- **Unit:** RunLogger con datos mock, cálculo de costo
- **Integration:** CRUD de runs + logs en DB real
- **Gherkin:** Escenarios del hu.md
- **Sabotaje:** Run sin logs → retorna array vacío, no error
