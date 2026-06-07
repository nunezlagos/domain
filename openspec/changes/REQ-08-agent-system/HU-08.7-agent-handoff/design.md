# Design: HU-08.7-agent-handoff

## Decisión arquitectónica

**Mecanismo:** tool-call `handoff_to_X` que muta `current_agent_id` y rompe la iteración (next iter usa agente nuevo).
**History:** compartido y read-only entre agentes en el mismo run.
**Limit:** max 5 handoffs por run; detección de loops por pattern últimos 4.

## Alternativas descartadas

- **Handoff implícito via heurística LLM:** impredecible, hard a auditar
- **Restart run con nuevo agente:** pierde history, mala UX
- **Handoff stateful (estado del agente A persistido):** complejidad alta sin ROI claro

## Schema diff

```sql
ALTER TABLE agents
  ADD COLUMN handoff_targets TEXT[] DEFAULT '{}',
  ADD COLUMN max_handoffs_per_run INT DEFAULT 5;

ALTER TABLE agent_runs
  ADD COLUMN current_agent_id UUID REFERENCES agents(id);

CREATE TABLE agent_run_handoffs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  from_agent_id UUID NOT NULL REFERENCES agents(id),
  to_agent_id UUID NOT NULL REFERENCES agents(id),
  reason TEXT,
  payload JSONB,
  occurred_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX ON agent_run_handoffs (agent_run_id, occurred_at);
```

## Engine flow

```
loop:
  agent = LoadAgent(run.current_agent_id)
  tools = PrepareTools(agent, run)  -- includes delegate_to_* and handoff_to_*
  resp = LLM.Chat(agent.system_prompt, run.messages, tools)
  for tool_call in resp.tool_calls:
    if startswith "handoff_to_":
      target = parseSlug(tool_call.name)
      validate target in agent.handoff_targets else 403
      check handoff_count + 1 <= max
      check no loop in last 4 handoffs
      INSERT agent_run_handoffs (from=agent.id, to=target.id, reason, payload)
      UPDATE agent_runs SET current_agent_id = target.id
      append message "Transferred to <target.name>"
      break  -- next iter uses new agent
    elif startswith "delegate_to_":
      ...
    else:
      executeSkill()
```

## TDD plan

1. Handoff con payload OK
2. Chain A→B→C
3. 6to handoff → 429
4. Loop pattern A→B→A→B → 409 "loop detected"
5. Audit timeline por agent
6. Cost split correcto por agente que generó cada tool_call
