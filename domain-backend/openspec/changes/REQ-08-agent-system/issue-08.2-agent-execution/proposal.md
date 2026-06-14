# Proposal: issue-08.2-agent-execution

## Intención

Construir el motor de ejecución de agentes: recibe un agent_id + input, carga la definición completa del agente, construye el contexto, invoca al LLM con tool calling (skills), maneja skill invocations cíclicamente, y retorna el output final con log.

## Scope

**In scope:**
- `AgentExecutor.Execute(ctx, agentID, input) -> RunResult`
- Flujo: Load Agent → Build Context → Init Run → LLM Call → Skill Loop → Finalize
- Tool calling: interceptar requests de tool del LLM → ejecutar skill → feed back
- Run record creation (status tracking) integrado con issue-08.3
- Integración con context optimizer (issue-07.1), token budget (issue-07.4)

**Out of scope:**
- Multi-agent orchestration (issue-08.4)
- Agent templates instantiation (issue-08.5)
- Streaming output (aunque el LLM puede stream, el executor manejaría el stream)

## Enfoque técnico

- `AgentExecutor` como orchestrator: usa `AgentService.GetAgent()`, `ContextBuilder`, `LLMProvider`, `SkillExecutor`
- Flujo:
  1. Load agent definition (model, prompt, skills, budget)
  2. Build context: system prompt + memories (via 07.1) + knowledge + skill registry (como tools)
  3. Create run record (status=running)
  4. Call LLM with context + tools (skills como tool definitions)
  5. If LLM returns tool_call → execute skill → append result → call LLM again
  6. Loop step 5 hasta que LLM retorna respuesta final o max iterations
  7. Finalize run (status=completed/failed, tokens, cost)

## Riesgos

- Skill loop infinito (LLM pide tool una y otra vez) → max_iterations (default 10)
- Token budget compartido entre múltiples skill invocations → cada skill call consume del mismo budget
- Skills pueden fallar → el executor debe decidir: retry, skip, o retornar error al LLM

## Testing

- **Unit:** Executor con agent mock, LLM mock, skill mock
- **Integration:** Executor + LLM real + skill real
- **Gherkin:** Escenarios del hu.md
- **Sabotaje:** Skill loop nunca termina → max_iterations lo corta
