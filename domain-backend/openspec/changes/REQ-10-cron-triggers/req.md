# REQ-10-cron-triggers: Automatización: cron schedules, webhooks (git push, PR), event-driven execution, trigger history, delivery guarantees.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F3, F5

## Descripción

Automatización: cron schedules, webhooks (git push, PR), event-driven execution, trigger history, delivery guarantees.

## Criterios de éxito

- CRUD de cron schedules con expresión cron estándar, timezone, scheduler worker que evalúa cada minuto y conserva execution history
- Webhook receiver con verificación HMAC-SHA256, mappers para GitHub/GitLab/genérico, delivery logs y replay manual
- Event bus pub/sub con suscripciones filtrables, entrega at-least-once con retry exponencial y dead letters
- Outbound webhooks: suscribir endpoints HTTP a events Domain, HMAC SHA-256 signing, retry exponencial 8 attempts + DLQ, circuit breaker, filters JSONPath, SSRF prevention

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-10.1-cron-schedules | proposed | Cron schedule CRUD, scheduler worker cada minuto, timezone support, execution history |
| issue-10.2-webhook-triggers | proposed | Webhook receiver HMAC-SHA256, GitHub/GitLab/generic mappers, delivery logs, replay |
| issue-10.3-event-execution | proposed | Event bus pub/sub, suscripciones con filtros, entrega at-least-once con retry, dead letters |
| issue-10.4-outbound-webhooks | proposed | Outbound webhooks subscriptions, HMAC, retry 8x + DLQ, circuit breaker, filters, replay, test ping |
