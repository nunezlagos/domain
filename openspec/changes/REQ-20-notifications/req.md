# REQ-20-notifications: Notificaciones outbound: abstracción de canales (email SMTP, Slack, webhook genérico) reutilizable por alertas, invitaciones, runs fallidos.

**Estado:** activo
**Creado:** 2026-06-06
**Fase:** F3

## Descripción

Sistema unificado de notificaciones outbound. Una abstracción `NotificationChannel` que adaptan los canales concretos (email SMTP, Slack webhook, webhook genérico). Consumido por usage alerts (HU-15.3), invitaciones (HU-21.2), runs fallidos, audit events. Templating con variables, delivery logs y retry.

## Criterios de éxito

- Interfaz `NotificationChannel` con métodos `Send(ctx, message)` implementada por al menos email-SMTP, Slack-webhook y webhook-genérico
- Templates de mensajes versionados con variables tipadas, render-only y render-and-send
- Delivery logs persistidos con status (sent, failed, retrying), latency, response code
- Retry con backoff exponencial, max attempts configurable, dead-letter después de N fallos
- Routing por evento + preferencia de usuario (default channel por user/org)

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-20.1-channel-abstraction | proposed | Interfaz NotificationChannel, registry, templates, delivery logs, retry |
| HU-20.2-email-smtp | proposed | Canal email SMTP con DKIM/SPF/TLS, soporte plantillas HTML/text |
| HU-20.3-slack-webhook | proposed | Canal Slack incoming webhook + Slack Block Kit messages |
