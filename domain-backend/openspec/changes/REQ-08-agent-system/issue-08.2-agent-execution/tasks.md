# Tasks: issue-08.2-agent-execution

## Backend

- [x] Implementar `AgentExecutor` struct con método `Execute(ctx, agentID, input)`
- [x] Implementar flujo base: LoadAgent → BuildContext → CreateRun → CallLLM → Finalize
- [x] Implementar `ContextBuilder` que integra: system prompt + memories (via issue-07.1) + skills registry
- [x] Implementar tool loop: detectar tool_calls en respuesta LLM → ejecutar skill → feed back
- [x] Implementar límite de iteraciones (max_iterations=10)
- [x] Integrar con `TokenBudgetManager` (issue-07.4) para tracking durante todo el run
- [x] Integrar con `SkillExecutor` (issue-05.5) para ejecución de skills
- [x] Integrar con `RunLogger` (issue-08.3) para logging de cada step
- [x] Manejar errores: modelo no disponible, skill falla, budget excedido
- [x] Exponer endpoint POST /agents/:id/run que invoca al executor

## Tests

- [x] Test unitario: ejecución básica retorna output + status=completed
- [x] Test unitario: skill invocation durante ejecución
- [x] Test unitario: error de modelo → run status=failed
- [x] Test unitario: max_iterations alcanzado → run status=failed
- [x] Test unitario: budget excedido durante tool loop → truncamiento
- [x] Test de integración: executor + LLM provider mock + skill mock
- [x] Test E2E: escenarios Gherkin del hu.md
- [x] Sabotaje: tool_call infinito → max_iterations lo corta

## Cierre

- [x] Verificación manual: ejecutar agente real con skills reales
- [x] Suite verde completa
- [x] Documentar flujo de ejecución y configuración de límites
