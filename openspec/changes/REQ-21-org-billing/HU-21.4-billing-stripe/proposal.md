# Proposal: HU-21.4-billing-stripe

## Intención

Integración Stripe vía Checkout (no Elements custom) + Customer Portal + Webhooks para upgrade/downgrade, pagos recurrentes, dunning y self-service billing management.

## Scope

**Incluye:**
- Stripe Go SDK
- Endpoints: checkout-session, portal-session, cancel, webhook
- Tabla `subscriptions` espejo del estado Stripe
- Webhook idempotency vía `stripe_event_id` único
- Dunning automatizado (3 fallos → downgrade)
- Sync periódico opcional (failsafe contra missed webhooks)

**No incluye:**
- Marketplace tax (Stripe Tax o manual)
- Usage-based billing (post-MVP)
- Multi-currency (USD-only inicialmente)

## Enfoque técnico

1. Checkout Sessions con `mode=subscription`, `success_url`, `cancel_url`
2. Webhook handler: validar signature, parsear evento, despachar handler por tipo
3. Idempotency: tabla `stripe_events_processed` con UNIQUE
4. Subscriptions table como snapshot eventual

## Riesgos

- Webhook secret leak → secret en vault, rotación
- Eventos out-of-order → trust Stripe `created` timestamp + reconciliation diaria
- Stripe API breaking changes → SDK pin, cambios anunciados

## Testing

- Mock Stripe Checkout flow end-to-end con stripe-mock
- Webhook signature válida/inválida
- 3 invoice.payment_failed → downgrade
- Cancel at period end → downgrade ocurre tras fecha
