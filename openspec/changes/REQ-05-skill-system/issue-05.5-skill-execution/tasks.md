# Tasks: issue-05.5-skill-execution

## Backend

- [ ] Crear migración para tabla `execution_logs` + índices
- [ ] Implementar interface `Executor` con 4 implementaciones
- [ ] Implementar PromptExecutor: render template + llamada LLM
- [ ] Implementar CodeExecutor: render + sandbox execution
- [ ] Implementar ApiExecutor: render URL/headers + HTTP call
- [ ] Implementar McpToolExecutor: render + MCP tool call
- [ ] Implementar resolución de versión (pinned vs latest)
- [ ] Implementar validación de parámetros contra JSON Schema
- [ ] Implementar handler POST /api/skills/:id/execute
- [ ] Implementar handler GET /api/executions/:id
- [ ] Implementar worker pool para ejecución async
- [ ] Implementar timeout con context.WithTimeout
- [ ] Implementar scrubbing de secretos en logs

## Frontend

- [ ] N/A (solo API)

## Tests

- [ ] Test unitario: PromptExecutor render y call mock
- [ ] Test unitario: CodeExecutor ejecución y captura de output
- [ ] Test unitario: ApiExecutor HTTP call y manejo de errores
- [ ] Test unitario: McpToolExecutor MCP call
- [ ] Test unitario: resolución de versión pinned vs latest
- [ ] Test unitario: validación de parámetros contra schema
- [ ] Test unitario: timeout cancela ejecución
- [ ] Test integración: ejecución sync completa
- [ ] Test integración: ejecución async + polling
- [ ] Test integración: log de ejecución tiene todos los campos
- [ ] Sabotaje: template inválido → error graceful, no panic
- [ ] Sabotaje: timeout forzado → success=false

## Cierre

- [ ] Verificación manual con curl
- [ ] Suite verde
