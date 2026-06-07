# Design: HU-08.6-multi-agent-supervisor

## Decisión arquitectónica

**Patrón:** delegate como tool-call (no handoff de control). El supervisor mantiene la sesión y el LLM ve cada resultado de subordinado como tool_result.
**Jerarquía:** árbol con `parent_run_id`; depth máximo 5.
**Budget:** propagación strict (`min(parent_remaining, delegate_hint)`).
**Cancel:** context cancellation cascada vía `errgroup` + ctx compartido.

## Alternativas descartadas

- **Handoff completo (estilo Swarm puro):** complica logging y user experience; el supervisor pierde control y se vuelve difícil aplicar policies
- **Sub-agente como skill normal:** no diferencia conceptual; pierde tree visualization, budget hierarchy y cancel semantics
- **MQ/queue entre supervisor y sub:** overkill para ejecuciones in-process; sub-runs deben ser sincrónicos para que el LLM espere

## Schema diff

```sql
ALTER TABLE agents
  ADD COLUMN subordinates TEXT[] DEFAULT '{}';

ALTER TABLE agent_runs
  ADD COLUMN parent_run_id UUID REFERENCES agent_runs(id),
  ADD COLUMN delegated_by_message_index INT,
  ADD COLUMN delegation_instructions TEXT,
  ADD COLUMN cancellation_reason VARCHAR(100);

CREATE INDEX ON agent_runs (parent_run_id) WHERE parent_run_id IS NOT NULL;
```

## Tool injection (pseudocódigo)

```go
func PrepareTools(agent *Agent, run *AgentRun) []Tool {
  tools := skills.AsTools(agent.Skills)
  for _, subSlug := range agent.Subordinates {
    sub := MustLoadAgent(subSlug)
    tools = append(tools, Tool{
      Name: "delegate_to_" + subSlug,
      Description: fmt.Sprintf("Delegate task to '%s': %s", subSlug, sub.Description),
      InputSchema: DelegateInputSchema(),
    })
  }
  return tools
}
```

## Delegate flow

```
1. LLM emits tool_use delegate_to_X with {instructions, context_keys}
2. Engine validates X in agent.Subordinates → else 403
3. Engine checks parent.ChainDepth() < 5 → else MaxDepthExceeded
4. Engine creates child agent_run with:
   - parent_run_id = current run id
   - agent_id = X
   - input_context = filter(parent.context, context_keys)
   - budget_remaining = min(parent_budget - parent_consumed, delegate_hint)
5. Engine runs child synchronously (same goroutine tree)
6. On child completion: tool_result returned to supervisor LLM
7. Continue supervisor loop
```

## Tree query

```sql
WITH RECURSIVE tree AS (
  SELECT *, 0 AS depth FROM agent_runs WHERE id = $1
  UNION ALL
  SELECT r.*, t.depth+1 FROM agent_runs r JOIN tree t ON r.parent_run_id = t.id
)
SELECT * FROM tree ORDER BY depth, created_at;
```

## TDD plan

1. Delegate tool generado por agent con subordinates
2. Delegate happy path: child run creado, output devuelto
3. Budget remaining decrece y se respeta
4. Cancel padre cancela hijos via ctx
5. Subordinado no autorizado → 403
6. Depth 6 → reject
7. Tree query devuelve árbol ordenado
8. Trace OTel parent-child span
