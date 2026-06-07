# RFC 0001: Agent vs Flow Boundary

**Status:** accepted
**Date:** 2026-06-07
**Author:** Domain Architecture
**Supersedes:** —
**Related:** REQ-08 Agent System, REQ-09 Flow System

## Contexto

Domain tiene dos sistemas de orquestación coexistiendo:

- **REQ-08 Agent System**: agentes LLM-driven que en runtime deciden qué tools/skills invocar. Loop dinámico.
- **REQ-09 Flow System**: DAGs declarativos (YAML/JSON) con state machine, retry policies, sub-flows. Pre-determinado.

Sin una boundary clara surgen preguntas que bloquean implementación:

- ¿Un Agent puede invocar un Flow?
- ¿Un Flow step `agent_run` cómo accounted?
- ¿Quién es el "outer container"?
- ¿Logging y cost accounting siguen qué jerarquía?
- ¿Una skill puede invocar un Flow?

## Decisión

Establecemos la jerarquía **Flow > Agent > Skill** con las siguientes reglas:

### Jerarquía y semántica

```
┌─────────────────────────────────────────────────────────┐
│ FLOW (declarative, durable, replayable, versionable)    │
│   ┌──────────────────────────────────────────┐          │
│   │ AGENT (LLM-driven loop, dynamic)         │          │
│   │   ┌──────────────────────────┐           │          │
│   │   │ SKILL (atomic, typed)    │           │          │
│   │   └──────────────────────────┘           │          │
│   └──────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────┘
```

### Reglas

1. **Flow es el outer container**. Si una request entra por trigger (cron/webhook/API), se modela preferentemente como Flow. Si entra como conversación LLM directa, se modela como Agent.
2. **Flow puede invocar Agent** vía step `agent_run` (HU-09.2 ya lo define).
3. **Flow puede invocar Skill** vía step `skill_call` (HU-09.2).
4. **Agent puede invocar Skill** vía tool-calling estándar (HU-08.2 + HU-05.6).
5. **Agent NO puede invocar Flow directamente.** Si necesita orquestación declarativa, debe declarar una skill `start_flow(flow_slug, inputs)` cuyo execute crea un nuevo flow_run y devuelve handle. La skill DEBE estar explícitamente en sus skills.
6. **Skill NO puede invocar Flow.** Skill puede invocar otra Skill (HU-05.6 `depends_on`) o sub-step interno, pero levantar un flow es responsabilidad del Flow engine.
7. **Sub-flows** (HU-09.5) sólo son invocables desde Flow steps.

### Cost & logging accounting

| evento | parent |
|--------|--------|
| Flow ejecuta `agent_run` step | costo va a `agent_runs.cost`, agregado al `flow_runs.cost` |
| Agent invoca skill | costo va a `agent_runs.cost` (skill no tiene costo propio aparte) |
| Skill llama otra skill | costo agregado al `agent_run` raíz |
| Skill `start_flow` desde Agent | nuevo flow_run con `triggered_by_agent_run_id`; costos separados pero linkados |

### Durability y replay

- Flows **siempre** son durables (HU-09.6) y versionados (HU-09.7).
- Agent runs son durables vía heartbeat + checkpoint en `agent_messages`, pero el "spec" del agent puede cambiar en runtime; usar `agent.version_id` para snapshot.
- Skills se snapshootean por `skill_version_id` en cada invocation.

### Cancelación

- Cancelar Flow cascadea a todos sus child agent_runs y sub_flow runs vía context cancel.
- Cancelar Agent cascadea a su tree de delegate/handoff (HU-08.6/7) y a flows que él disparó vía `start_flow` SOLO si declara `cascade_started_flows: true`.

### Triggers

| trigger | landing |
|---------|---------|
| Cron (REQ-10) | Flow run |
| Inbound webhook (HU-10.2) | Flow run |
| User chat (HU-12.x MCP) | Agent run |
| API POST /api/v1/runs | Either, explicit `kind: flow|agent` |
| External signal (HU-09.8) | Flow only |

## Alternativas consideradas

### Alternativa A: Agent puede invocar Flow directamente (rechazada)

Permitir tool sintético `start_flow` implícito en cualquier agent. **Rechazada** porque:
- Hace impredecible qué Flows puede correr un agent
- Dificulta RBAC enforcement
- Cualquier prompt injection podría disparar Flows arbitrarios

Mitigación adoptada: el agent debe **explicitar la skill** `start_flow` (con whitelist de flow_slugs) en su definition. Eso reusa el contrato Agent↔Skill (HU-05.6) y RBAC.

### Alternativa B: Sin jerarquía, ambos peer (rechazada)

Tratar Flow y Agent como sistemas hermanos sin parent-child. **Rechazada** porque:
- Sin parent → cost accounting ambiguo
- Sin parent → cancel cascade impreciso
- Sin parent → tree visualization imposible

### Alternativa C: Solo Flow (sin Agent) (rechazada)

Modelar todo como flow declarativo, eliminar Agent system. **Rechazada** porque:
- Pierde uso principal: chat conversacional LLM-driven
- Forza pre-declarar todas las decisiones que el LLM debería tomar
- LLM-as-orchestrator es un patrón legítimo

## Consecuencias

**Positivas:**
- Boundary clara: developers saben cuál usar para qué
- RBAC y cost accounting predictible
- Cancel/cleanup cascadea correctamente
- Compatible con HU-08.6 supervisor (que delega a sub-agents, no a flows)

**Negativas:**
- Agent que necesita "siguiente paso determinístico" debe declarar skill `start_flow` (más boilerplate)
- Dos sistemas a mantener (vs unificar) — aceptado: cubren casos distintos

## Implementación

- HU-08 agentes NO agregar tool implícito `start_flow`
- HU-05 skills: skill `start_flow` debe estar en skill registry como built-in opcional
- HU-09 flow_runs.triggered_by_agent_run_id columna agregada
- HU-09 flow_runs.cascade_started_flows BOOL en Agent config
- Cost reporting tree muestra ambas hierarchies linkadas

## Open questions

- ¿Cómo se modela conversational agent que tras 10 turns dispara batch flow? — Por la skill `start_flow`. El cost accounting separa pero linkea.
- ¿Si un Flow contiene `agent_run` step y ese agent dispara otro flow vía skill, hasta dónde cascadea cancel? — Convenir cap depth 3 niveles de mix Flow↔Agent.
