# REQ-33 — SaaS Protection (operacional, no comercial)

> **Origen**: sesión 2026-06-12. Decisión explícita del usuario:
> NO HAY tiers premium / Stripe / paywall. Solo tracking de uso para
> métricas internas y protección contra runaway de clientes. La
> distinción importante: rate limit per-org es PROTECCIÓN
> OPERACIONAL contra abuso accidental (un cliente con un loop que
> baja el VPS), no un mecanismo comercial.

## Contexto

Hoy el harness MCP tiene rate limit global (120/min default, 60/min
mutations). Eso significa que UN cliente abusivo (intencional o por
bug) puede afectar a TODOS los demás clientes. Sin protección
per-org, el primer cliente con un script en loop te baja el VPS.

Lo MISMO con costos LLM: un cliente con un agente que entra en loop
chamando Anthropic te funde el presupuesto. No es "le facturamos más"
— es "te avisamos y vos decidís si cortar el cliente".

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 33.1 | `rate-limit-per-org` | M | Mover el rate limit del harness MCP de scope global a per-org. Token bucket per `Principal.OrganizationID`. Default 1000 req/min por org. Configurable per-org (no per-plan — no hay planes). Header `X-RateLimit-*` en response. |
| 33.2 | `cost-tracking-soft-alerts` | M | Tracking ya existe en `cost_logs`. Agregar: alerta soft (email a admin) si org gasta > X USD/día. X configurable per-org (default 100 USD). NO bloquea — solo notifica. Job cron horario que consulta `cost_logs` y emite alertas. |
| 33.3 | `max-flow-duration-per-org` | S | Guardrail: si un flow_run excede `MaxFlowDuration` de su org (default 5min), cancelar. Configurable per-org. Protege al scheduler compartido de un cliente con flows infinitos bloqueando goroutines. |
| 33.4 | `quota-snapshot-dashboard-ready` | S | Endpoints `/api/v1/usage/current` y `/api/v1/usage/history` para que el dashboard (futuro) muestre al cliente cuánto consumió. Solo lectura, NO cobro. |

## Prioridad: **media** (antes del primer cliente externo)

No urgente para el primer despliegue. Pero ANTES de aceptar clientes
externos, sin esto, el primer cliente con bug te puede arruinar el
servicio para todos.
