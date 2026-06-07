# Design: HU-08.4-multi-agent-orch

## Decisión arquitectónica

**Patrón:** Supervisor/Worker con delegación explícita vía tool calling.

```
Supervisor Agent (llama)
    │
    │ "Necesito que revises este código"
    │ tool_call: delegate_to_agent(agent="code-reviewer", input="...")
    │
    ▼
AgentOrchestrator
    │
    ├── Check max_depth, detect cycles
    ├── Create sub-run (linked to parent)
    ├── Execute AgentExecutor(code-reviewer, input)
    ├── Collect result
    │
    ▼
Supervisor (recibe resultado como tool_response)
    │
    │ "El revisor encontró 3 issues. Ahora..."
    │
    ▼
Continúa ejecución
```

## Alternativas descartadas

1. **Comunicación directa entre agentes (peer-to-peer):** Difícil de controlar y monitorear. El orquestador central es necesario.
2. **Protocolo de mensajería async (colas):** Overkill para MVP. La delegación es síncrona (el supervisor espera).
3. **Graph-based DAG de agentes (como flujos):** Sería REQ-09, no esto. Acá es dinámico: el supervisor decide en runtime.

## Diagrama

```
┌─────────────────────────────────────────────────────────┐
│ AgentOrchestrator                                        │
│                                                          │
│  RunTree:                                                 │
│  ┌─ Run #1 (supervisor: "Architect")                     │
│  │   ├─ SubRun #1.1 (worker: "Code Reviewer") [completed] │
│  │   ├─ SubRun #1.2 (worker: "Bug Hunter")  [completed]  │
│  │   └─ SubRun #1.3 (worker: "PR Describer") [running]   │
│  │                                                       │
│  Config:                                                  │
│  ├─ MaxDepth: 3                                          │
│  ├─ MaxConcurrent: 5                                     │
│  └─ Timeout: 120s por subagente                          │
└─────────────────────────────────────────────────────────┘
```

## TDD plan

1. **Red:** Test supervisor delega a subagente → subagente se ejecuta y retorna resultado
2. **Green:** Implementar `Delegate()` con AgentExecutor interno
3. **Refactor:** Agregar paralelismo, depth check, error handling
4. **Sabotaje:** Ciclo A→B→A → max_depth lo detecta y retorna error

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Delegación recursiva sin control | max_depth=3, detectar agent_id repetidos en el árbol |
| Ejecución paralela satura LLM API | max_concurrent=5, semáforo para limitar |
| Contexto grande entre saltos | Comprimir contexto delegado; usar resumen en vez de raw |
| Timeout de subagente | Timeout configurable por sub-run; si expira, error al supervisor |
| Deadlock (supervisor espera subagente que espera supervisor) | Detección de dependencia circular en el árbol de runs
