# Proposal: issue-08.4-multi-agent-orch

## Intención

Implementar un `AgentOrchestrator` que permita a un agente supervisor delegar subtareas a otros agentes, con protocolo de handoff, ejecución paralela y manejo de errores.

## Scope

**In scope:**
- `AgentOrchestrator` con métodos: `Delegate(ctx, fromAgent, toAgent, input, context)`, `ParallelRun(ctx, agents, inputs)`
- Handoff protocol: contexto estructurado + metadata de origen
- Skill especial `delegate_to_agent` que el supervisor invoca como un tool call
- Ejecución paralela via goroutines con waitgroup + recolección de resultados
- Manejo de errores: error en subagente → resultado con error → supervisor decide
- Tracking: cada delegación genera sub-runs vinculados al run padre

**Out of scope:**
- Detección automática de qué agente usar (el supervisor debe decidir explícitamente)
- Ciclos de delegación (A→B→A) → el orquestador debe detectarlos
- Priorización o scheduling complejo

## Enfoque técnico

- `DelegateToAgent` se expone como un skill más en el registry del supervisor
- `AgentOrchestrator.Delegate()`: crea un sub-run vinculado al run padre, ejecuta AgentExecutor en el subagente, retorna resultado
- Paralelismo: `ParallelRun()` lanza N goroutines, cada una ejecuta `Delegate()`, recolecta resultados en un channel
- Handoff: el contexto se serializa como JSON, se pasa como input al subagente (con metadata: `_delegated_from: agent_id, _parent_run_id`)
- Errores: el sub-run captura el error, lo devuelve como parte del resultado (no propaga excepción)

## Riesgos

- Delegación recursiva puede crear ciclos → max_depth configurable (default 3)
- Ejecución paralela puede saturar recursos → max_concurrent configurable
- Contexto muy grande pasado entre agentes → puede exceder budget del subagente

## Testing

- **Unit:** Orchestrator con agentes mock, delegación, paralelismo
- **Integration:** Orchestrator + AgentExecutor real con agentes reales
- **Gherkin:** Escenarios del hu.md
- **Sabotaje:** Ciclo A→B→A detectado por max_depth → error
