# Design: issue-08.2-agent-execution

## Decisión arquitectónica

**Patrón:** Orchestrator con loop de tool calling (ReAct-like).

```
AgentExecutor.Execute()
    │
    ├─ 1. LoadAgent(agentID) → Agent
    ├─ 2. BuildContext(agent) → Context (system + memories + skills)
    ├─ 3. CreateRun(agent, input) → Run (status=running)
    │
    └─ Loop (max 10 iterations):
        ├─ 4. CallLLM(context + tool_definitions) → Response
        ├─ 5. If tool_call:
        │      ├─ ExecuteSkill(skill, args) → Result
        │      ├─ AppendResultToContext(result)
        │      └─ Continue loop
        └─ 6. If text_response → Finalize
                      │
                      ▼
              FinalizeRun(output, tokens, cost, status=completed)
```

## Alternativas descartadas

1. **Ejecución síncrona sin tool loop:** No permite uso de skills. Descartado porque los skills son centrales.
2. **Cada skill en proceso separado:** Overkill para MVP. Skills se ejecutan in-process con sandbox opcional (REQ-11).
3. **State machine externa (como REQ-09 flow):** Demasiado pesado para un agente simple. El loop es suficiente.

## Diagrama

```
┌──────────────┐     ┌───────────────┐     ┌─────────────┐
│ AgentService │     │ AgentExecutor │     │ ContextBldr │
│ (definición) │────▶│ (orquestador) │────▶│ (07.1)      │
└──────────────┘     │               │     └─────────────┘
                     │  ┌──────────┐ │     ┌─────────────┐
                     │  │ LLM Call │ │◀───▶│ LLMProvider │
                     │  │ (tool)   │ │     │ (06.1)      │
                     │  └──────────┘ │     └─────────────┘
                     │       │       │     ┌─────────────┐
                     │  ┌──────────┐ │     │ SkillExec   │
                     │  │ Skill    │ │◀───▶│ (05.5)      │
                     │  │ Loop     │ │     └─────────────┘
                     │  └──────────┘ │     ┌─────────────┐
                     │               │     │ TokenBudget │
                     │               │────▶│ (07.4)      │
                     └──────────────┘     └─────────────┘
```

## TDD plan

1. **Red:** Test ejecución básica retorna RunResult con output y status=completed
2. **Green:** Implementar flujo lineal Load → Build → Run → LLM → Finalize (sin tool loop)
3. **Refactor:** Agregar tool loop con skill execution
4. **Sabotaje:** LLM devuelve tool_call infinitos → max_iterations lo corta con status=failed

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Tool loop infinito | max_iterations=10, si se excede → failed con error "max_iterations_reached" |
| Skill falla durante ejecución | Capturar error, pasarlo como tool_response al LLM para que decida |
| Token budget compartido entre iterations | Usar mismo TokenBudgetManager a través del loop |
| Contexto crece con cada tool response | Monitorear budget después de cada skill execution; si se agota, truncar y advertir al LLM
