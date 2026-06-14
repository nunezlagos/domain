# Proposal: issue-20.1-channel-abstraction

## Intención

Abstracción `NotificationChannel` con registry, templating, delivery logs y retry policy reutilizable por todas las features que necesiten notificar.

## Scope

**Incluye:**
- Interfaz `Channel` + registry singleton
- Sistema de templates Go con strict mode (error en variable faltante)
- Tabla `notification_templates`, `notification_subscriptions`, `notification_deliveries`
- Worker async que procesa cola de envíos con retry exponencial
- Routing por evento + subscription preferences
- Idempotencia con dedup key opcional

**No incluye:**
- Canales concretos (issue-20.2, issue-20.3)
- UI de gestión (parte de REQ-16 si aplica)

## Enfoque técnico

1. Cola en `notification_deliveries` con status pending → processing → sent/failed
2. Worker pool con N workers, claim con `FOR UPDATE SKIP LOCKED`
3. Templates en tabla con versionado (mismo patrón que prompts)
4. Subscriptions configurables a nivel org y user
5. Tabla `notification_optouts` para opt-out por categoría

## Riesgos

- Loops de notificación (alerta de fallo de notificación → fallo → notificación...): max-depth check
- Spam: rate-limit por (channel, recipient, template)
- PII en logs: campos `content` enmascarados en `notification_deliveries`

## Testing

- Interface mock + tests channel registry
- Template render con variables OK/missing
- Retry: simular fallos transitorios → reintenta
- Dead letter después de N intentos → notifica admin canal
