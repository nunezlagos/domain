# issue-08.3-agent-runs-logs

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** operador del sistema de agentes
**Quiero** ver el historial completo de ejecuciones de un agente con estado, input, output, tokens usados, costo, duración, errores, y log detallado de cada LLM call y skill execution dentro del run
**Para** auditar, debuggear y optimizar el comportamiento de los agentes

## Criterios de aceptación

### Scenario 1: Log de run completado
**Given** un agente que ejecutó un run exitosamente
**When** se consulta GET /agents/:id/runs
**Then** retorna la lista de runs con: id, agent_id, project_id, status, input, output, tokens_used, cost, started_at, ended_at

### Scenario 2: Log de run fallido
**Given** un run que falló por error de modelo
**When** se consulta GET /runs/:run_id
**Then** retorna status=failed
**And** error = "model_not_available"
**And** el output es vacío (o parcial si truncado)
**And** tokens_used refleja lo consumido antes del error

### Scenario 3: Log detallado de LLM calls
**Given** un run que incluyó 3 LLM calls (1 inicial + 2 tool follow-ups)
**When** se consulta GET /runs/:run_id/log
**Then** retorna 3 entradas de log con timestamp, tipo=llm_call, prompt, response, tokens, duración
**And** cada entrada incluye el tool_call y tool_response si corresponde

### Scenario 4: Log de skill executions
**Given** un run que ejecutó 2 skills (list-files, read-file)
**When** se consulta GET /runs/:run_id/log
**Then** retorna 2 entradas con tipo=skill_execution, skill_name, args, result, duration, status

### Scenario 5: Filtrado y paginación
**Given** 50 runs de un agente
**When** se consulta GET /agents/:id/runs?status=completed&limit=10&offset=0
**Then** retorna los primeros 10 runs completados
**And** incluye header X-Total-Count: 35 (total completados)

## Análisis breve

- **Qué pide realmente:** Sistema de logging estructurado para runs de agente que capture cada LLM call y skill execution con metadata (tokens, costo, duración) y permita consulta con filtros y paginación.
- **Módulos sospechados:** `internal/agent/`, `internal/runner/`, `internal/logging/`
- **Riesgos / dependencias:** Depende de agent execution (08.2) que produce los logs, y del tracking de costo (issue-06.4, issue-15.1).
- **Esfuerzo tentativo:** M**
