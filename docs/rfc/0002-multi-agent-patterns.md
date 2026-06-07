# RFC 0002: Multi-Agent Patterns

**Status:** accepted
**Date:** 2026-06-07
**Related:** HU-08.6 supervisor, HU-08.7 handoff, HU-08.8 parallel-fanout, HU-08.9 hierarchical-context

## Contexto

OpenAI Swarm, Anthropic sub-agents, LangGraph, CrewAI, AutoGen exploran multi-agent. Cada uno toma decisiones distintas (handoff vs delegate, memoria compartida vs aislada, paralelismo vs secuencial). Domain debe elegir explícitamente qué patrones soporta y cómo.

## Decisión

Domain soporta TRES patrones canónicos, **complementarios y combinables**, cada uno con HU dedicada:

### Patrón 1: Supervisor + Delegate (HU-08.6)

```
A (supervisor)
  ├─ delegate_to_B → B → output → continúa
  └─ delegate_to_C → C → output → finaliza
```

- A invoca a B/C como **tool-call** (LLM emite, motor crea sub-run)
- A **mantiene control** y ve los outputs como tool_results
- A puede invocar mismo subordinate múltiples veces en distintos turns
- Budget hierarchy: child ≤ remaining(parent)
- Cancel cascadea down

**Usar cuando:** A necesita combinar/comparar/iterar sobre outputs de subordinados.

### Patrón 2: Handoff (HU-08.7)

```
A (triage)
  └─ handoff_to_B → B continúa la conversación
                     └─ handoff_to_C → C continúa
```

- A "transfiere" la sesión a B mid-flight
- B usa su propio system_prompt + skills
- Historia compartida read-only
- Max 5 handoffs en cadena
- Loop detection

**Usar cuando:** routing/triage; un agent decide que B es más apropiado y se sale del medio.

### Patrón 3: Parallel Fan-out (HU-08.8)

```
A (supervisor)
  └─ parallel_fanout([B, C, D], strategy="majority_vote")
      ├─ B → output_b
      ├─ C → output_c     ─→ merge → tool_result
      └─ D → output_d
```

- N sub-agents corriendo concurrentes (max 10 default)
- Merge strategies built-in: first_completed, all_results, majority_vote, best_of_n, reduce_skill
- Budget pool compartido
- Cancel cascadea down
- Partial results si timeout

**Usar cuando:** ensemble, paralelismo para reducir latencia, voting.

## Patrones combinables

```
Supervisor A
  ├─ parallel_fanout([B, C], majority_vote)
  │   ├─ B → delegate_to_E → E
  │   └─ C → handoff_to_F → F
  └─ delegate_to_D → D
```

Cada child puede a su vez aplicar otro patrón. Depth máxima total: **5 niveles**.

## Memoria y contexto (HU-08.9)

Reglas para todos los patrones:

| Acción | Permitido |
|--------|-----------|
| Child lee scope `run` del parent | ✓ read-only |
| Child escribe a su own scope `run` | ✓ |
| Child escribe a scope `run` del parent | ✗ — sólo via `upstream_keys` declarados |
| Child lee scope `project`/`organization` | ✓ (si RBAC permite) |
| Child escribe scope `project`/`organization` | ✓ persistente |

**Bloat protection:** si parent no pasa `context_keys` explícitos, child no recibe state inicial; debe pedir vía tool `parent_memory_get`.

## Patrones que NO soportamos (explícitamente)

- **Peer-to-peer sin supervisor**: agents A, B, C charlan entre sí sin jerarquía. Difícil de auditar, costo no acotado. Si se necesita: modelar como supervisor con `parallel_fanout` y `reduce_skill`.
- **Self-modifying agents**: agente que rewrite su own system_prompt mid-run. Versionar en lugar.
- **Reflection loops infinitos** (agente se evalúa a sí mismo hasta converger): permitido pero capado por `max_iterations` por agent_run.

## Anti-patrones

| Anti-pattern | Por qué evitar | Alternativa |
|--------------|---------------|-------------|
| Delegate con `instructions = full conversation history` | Context bloat | Pasar `context_keys` específicos |
| Handoff chain de 4+ hops | UX confusa | Usar supervisor + delegate |
| Parallel fan-out de 20 agents | Cost & memory | Cap a 10, considerar batch |
| Sub-agent escribe arbitrariamente parent scope | Race conditions | upstream_keys declarados |

## Choosing the right pattern

```
¿El agente necesita combinar resultados de varios? → Supervisor + Delegate
¿El agente solo necesita pasar la conversación a otro? → Handoff
¿Necesito paralelismo o ensemble voting? → Parallel Fan-out
¿Combinación? → Componer (con cap depth 5)
```

## Métricas a publicar (REQ-17)

- `domain_agent_delegations_total{from, to}`
- `domain_agent_handoffs_total{from, to}`
- `domain_agent_fanout_total{strategy}`
- `domain_agent_chain_depth` histogram
- `domain_agent_subagent_failures_total{reason}`

## Open questions

- ¿Hacer falta patrón "request-reply async" donde child es llamado, devuelve placeholder, y completa después via signal? — TBD, deferir hasta tener caso concreto.
- ¿"Memory write-through" donde child escribe directo a `project` scope sin upstream? — Sí, ya cubierto: scope `project` es persistente y no necesita upstream merge.
