# Proposal: issue-08.7-agent-handoff

## Intención

Implementar el patrón handoff donde un `agent_run` cambia de agente activo mid-flight (un agente "transfiere" la conversación a otro), distinto de delegate (issue-08.6). Útil para triage y routing.

## Scope

**Incluye:**
- Columna `agents.handoff_targets TEXT[]` con slugs permitidos
- Columna `agent_runs.current_agent_id UUID` (siempre apunta al activo)
- Tabla `agent_run_handoffs` (log de transitions)
- Tool generator `handoff_to_<slug>` en runtime
- Max handoffs por run configurable (default 5)
- Conversation history compartido (read-only) entre agentes en el mismo run
- Audit + timeline por agente

**No incluye:**
- Multi-user handoff (transferir a otro user humano) — futuro
- Persistencia de "estado interno" del agente A cuando se va (cada agente arranca clean)

## Enfoque técnico

1. Engine al iniciar iteración: lee `current_agent_id` → carga agent.system_prompt + skills
2. Tool `handoff_to_X`: actualiza `current_agent_id`, inserta handoffs row, break iteration → next iter usa nuevo agent
3. History compartido: tabla `agent_run_messages` ya existe, no se filtra por agent
4. Loop detection: si en últimos 5 handoffs aparece patrón A→B→A→B → reject "loop detected"

## Riesgos

- Confusión usuario: documentar bien diferencia con delegate
- Loop: detección por pattern matching últimos N handoffs
- Cost accounting: split por agente correcto

## Testing

- Handoff con payload
- Chain A→B→C OK
- 6to handoff → 429
- Loop A→B→A→B detectado
- Audit timeline correcto
- Costo split por agente
