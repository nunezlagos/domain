# Tasks: HU-08.2-agent-execution

## Backend

- [ ] Implementar `AgentExecutor` struct con método `Execute(ctx, agentID, input)`
- [ ] Implementar flujo base: LoadAgent → BuildContext → CreateRun → CallLLM → Finalize
- [ ] Implementar `ContextBuilder` que integra: system prompt + memories (via HU-07.1) + skills registry
- [ ] Implementar tool loop: detectar tool_calls en respuesta LLM → ejecutar skill → feed back
- [ ] Implementar límite de iteraciones (max_iterations=10)
- [ ] Integrar con `TokenBudgetManager` (HU-07.4) para tracking durante todo el run
- [ ] Integrar con `SkillExecutor` (HU-05.5) para ejecución de skills
- [ ] Integrar con `RunLogger` (HU-08.3) para logging de cada step
- [ ] Manejar errores: modelo no disponible, skill falla, budget excedido
- [ ] Exponer endpoint POST /agents/:id/run que invoca al executor

## Tests

- [ ] Test unitario: ejecución básica retorna output + status=completed
- [ ] Test unitario: skill invocation durante ejecución
- [ ] Test unitario: error de modelo → run status=failed
- [ ] Test unitario: max_iterations alcanzado → run status=failed
- [ ] Test unitario: budget excedido durante tool loop → truncamiento
- [ ] Test de integración: executor + LLM provider mock + skill mock
- [ ] Test E2E: escenarios Gherkin del hu.md
- [ ] Sabotaje: tool_call infinito → max_iterations lo corta

## Cierre

- [ ] Verificación manual: ejecutar agente real con skills reales
- [ ] Suite verde completa
- [ ] Documentar flujo de ejecución y configuración de límites
