# REQ-35 — Architectural Debt (deuda técnica detectada)

> **Origen**: sesión 2026-06-12. Crítica honesta del agente al revisar
> la arquitectura de domain con la información del plan multi-tenant.
> Tres áreas con deuda concreta que se va a sentir cuando haya N
> clientes pero hoy es invisible.

## Contexto

Durante la sesión se revisó la arquitectura runtime de domain
(agents, skills, flows, crons, webhooks, MCP) y se detectó:

1. **3 dispatchers duplicados**: cron, webhook y MCP cada uno tiene su
   `switch target_type → flow/agent/skill runner`. Tres lugares para
   mantener consistencia. Bug típico: agregar un step_type nuevo y
   olvidarse de uno.

2. **Skill model sobrecargado**: 4 tipos declarados (`TypePrompt`,
   `TypeAPI`, `TypeCode`, `TypeMCPTool`) pero solo 2 implementados.
   El agente externo Claude/opencode ni siquiera usa skills
   server-side. Mucha estructura para entregar poco.

3. **Pelea de protocolo con engram via instructions**: el stub global
   dice "domain tiene prioridad" pero el LLM puede ignorarlo. Se
   gana técnicamente desactivando engram, no peleando instructions.

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 35.1 | `unified-dispatcher` | M | Crear `internal/dispatch.Dispatcher` con método `Dispatch(target_type, target_id, inputs, source)`. Reemplazar las 3 implementaciones actuales (`cronsched.dispatchSync`, `webhook.dispatchWebhook`, `mcp.handleFlowRun/handleAgentRun/handleSkillExecute`) por llamadas al dispatcher. Reduce 3 lugares con bugs potenciales a 1. |
| 35.2 | `skill-model-decision-record` | M | Diseño con tradeoffs: ¿simplificamos a `TypePrompt` único (matamos TypeAPI/TypeCode/TypeMCPTool) o committeamos a implementar los stubs para entregar valor SaaS real? Decisión arquitectónica con ADR formal. Si simplificación gana → retirar stubs en una migración + ajustar docs. Si implementación gana → REQ separado por cada tipo faltante. |
| 35.3 | `setup-primary-memory-detect-engram` | S | Comando `domain setup opencode --primary-memory` que escanea `~/.config/opencode/opencode.json` y `~/.claude.json`, detecta otros MCP servers de memoria (engram, mem0, conocidos), ofrece DESACTIVARLOS con backup. Resuelve técnicamente el "domain prioritario" que el stub de instructions intenta resolver lingüísticamente. |
| 35.4 | `runners-coverage-audit` | S | Revisar uso real de agent runner y flow runner server-side en producción (telemetría de los últimos 30 días post-launch). Si NUNCA se usaron, marcar como "beta — usá MCP directo" o mover a REQ separado deferido. Decisión basada en datos, no especulación. |

## Prioridad: **baja** (cuando haya aire)

Ninguno bloquea entrega de SaaS. Pero el día que la deuda se sienta
(cliente reporta "no funciona X" y el bug está duplicado en 3 lugares)
vale la pena.
