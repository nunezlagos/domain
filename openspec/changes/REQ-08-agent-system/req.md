# REQ-08-agent-system: Sistema de agentes: definiciones (modelo, provider, system prompt, skills asignados), ejecución con estado, runs, logs, multi-agent orchestration.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F2, F5

## Descripción

Sistema de agentes: definiciones (modelo, provider, system prompt, skills asignados), ejecución con estado, runs, logs, multi-agent orchestration.

## Criterios de éxito

- CRUD de agentes con slug, modelo, provider, system prompt, skills asignados y versionado
- Motor de ejecución que crea `agent_runs`, carga contexto (memoria + knowledge), invoca LLM y ejecuta skills en loop con tool-calling
- Logging completo de runs: estado, tokens IN/OUT, costo, traza de llamadas LLM y skill executions
- Orquestación multi-agente: supervisor delega a sub-agentes con handoff explícito y paralelismo cuando aplica
- Plantillas predefinidas listas: Code Reviewer, Architecture Advisor, Bug Hunter, PR Reviewer, Doc Writer
- Patrones multi-agent canónicos (HU-08.6/7/8/9): supervisor+delegate, handoff, parallel fan-out con merge strategies, hierarchical context con read-only inheritance y upstream_keys explícitos

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-08.1-agent-definitions | propuesta | CRUD de agentes con slug, modelo, system prompt, skills, versionado |
| HU-08.2-agent-execution | propuesta | Motor de ejecución: crea run, carga contexto, invoca LLM + skills en loop |
| HU-08.3-agent-runs-logs | propuesta | Logging de runs: estado, tokens, costo, LLM calls detalladas, skill executions |
| HU-08.4-multi-agent-orch | propuesta | Orquestación multi-agente: supervisor delega a subagentes con handoff y paralelismo |
| HU-08.5-agent-templates | propuesta | Plantillas predefinidas: Code Reviewer, Architecture Advisor, Bug Hunter, PR Reviewer |
| HU-08.6-multi-agent-supervisor | proposed | Supervisor + delegate as tool-call, child agent_runs jerárquicos, budget propagation, cancel cascade, tree view |
| HU-08.7-agent-handoff | proposed | Handoff explícito (transferir conversación), max 5 hops, loop detection, audit por agente |
| HU-08.8-agent-parallel-fanout | proposed | Parallel fan-out con 5 merge strategies (first/all/vote/best-of-N/reduce-skill), budget pool, timeout global |
| HU-08.9-agent-hierarchical-context | proposed | KV scoped (run/agent/project/org), read-only inheritance, upstream_keys explícitos, RBAC enforcement |
