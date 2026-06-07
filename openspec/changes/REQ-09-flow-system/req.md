# REQ-09-flow-system: Sistema de flujos: DAGs de pasos (skill_call, llm_call, code_exec, conditional, parallel, wait, human_input), state machine, retry policies, sub-flows.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F3, F5

## Descripción

Sistema de flujos: DAGs de pasos (skill_call, llm_call, code_exec, conditional, parallel, wait, human_input), state machine, retry policies, sub-flows.

## Criterios de éxito

- Definición de flujos como DAGs validados sin ciclos, con import/export en YAML y JSON
- 10 step types soportados: skill_call, llm_call, code_exec, conditional, parallel, wait, human_input, agent_run, sub_flow, transform
- State machine event-driven: pending → running → completed/failed/paused/cancelled, con transiciones auditadas
- Retry policies por step con backoff exponencial, estrategias ignore/abort/fallback y Dead Letter Queue
- Sub-flows como pasos con contexto padre→hijo, composición paralela y detección de circularidad
- Durable execution: checkpoint por step, heartbeat, recovery scanner, output overflow a S3, replay-safe flag
- Workflow versioning: cada save crea draft, publish activa, runs en vuelo congelados a su versión; diff json-patch
- External signals: step `await_signal` con timeout + retry; broadcast a múltiples runs; LISTEN/NOTIFY wake
- Saga compensation: declarar `compensate` por step; ejecución reversa con retry; manual skip endpoint
- Step heartbeats con progress visible vía SSE
- Reproducibility snapshots: frozen time + random seed + LLM cache → replay determinístico
- Dry-run plan mode: estima tokens y costo sin side-effects, detecta conditionals dinámicos

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-09.1-flow-dag-definition | proposed | Flow DAG CRUD, validación de ciclos, YAML/JSON import/export |
| HU-09.2-step-types | proposed | 10 step types: skill_call, llm_call, code_exec, conditional, parallel, wait, human_input, domain_agent_run, sub_flow, transform |
| HU-09.3-flow-state-machine | proposed | State machine de ejecución: pending→running→completed/failed/paused/cancelled, event-driven transitions |
| HU-09.4-retry-error-handling | proposed | Retry policies, backoff, ignore/abort/fallback, Dead Letter Queue |
| HU-09.5-subflows-composition | proposed | Sub-flows como pasos, contexto padre→hijo, parallel composition, detección de circularidad |
| HU-09.6-durable-execution | proposed | Checkpoint por step, heartbeat, recovery scanner, output S3 overflow, replay-safe flag |
| HU-09.7-workflow-versioning | proposed | Versionado draft→published→deprecated, runs en vuelo congelados, diff json-patch |
| HU-09.8-external-signals | proposed | Step await_signal con payload, timeout, broadcast multi-run, RBAC permission |
| HU-09.9-saga-compensation | proposed | Compensate por step, ejecución reversa, retry, manual skip, parallel mode opcional |
| HU-09.10-step-heartbeats | proposed | ctx.Heartbeat(progress) para steps largos, zombie detection, SSE progress events |
| HU-09.11-reproducibility-snapshots | proposed | Snapshot determinístico (seed, frozen time, LLM cache) + endpoint replay |
| HU-09.12-dry-run-plan-mode | proposed | Static analysis del flow + token/cost estimate sin side-effects |
