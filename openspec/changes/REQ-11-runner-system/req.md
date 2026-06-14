# REQ-11-runner-system: Ejecución en sandbox: runners cloud y self-hosted (Docker), execution streaming, logs, timeouts, resource limits.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F5

## Descripción

Ejecución en sandbox: runners cloud y self-hosted (Docker), execution streaming, logs, timeouts, resource limits.

## Criterios de éxito

- Sandbox Docker aísla código arbitrario con resource limits, timeout y network control
- Runner self-hosted se conecta, registra y ejecuta tareas localmente
- Cliente puede suscribirse a ejecuciones y recibir eventos en tiempo real (WS + SSE)

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-11.1-sandbox-execution | proposed | Sandbox Docker con resource limits, timeout, network isolation y GC |
| issue-11.2-selfhosted-runner | proposed | Agente Go self-hosted vía WebSocket con registro, heartbeat y ejecución local |
| issue-11.3-execution-streaming | proposed | Streaming bidireccional WS + SSE para logs, progreso y LLM tokens en vivo |
