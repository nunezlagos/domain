# Proposal: issue-20.3-slack-webhook

## Intención

Dos canales hermanos: `slack-webhook` (con Block Kit + rate limit Slack) y `webhook-generic` (POST genérico con HMAC opcional). Ambos comparten la lógica HTTP base.

## Scope

**Incluye:**
- Canal `slack-webhook` con Block Kit support
- Canal `webhook-generic` con headers configurables + HMAC opcional
- Rate limit interno por recipient (1 req/s por canal Slack)
- Validación URL (must https para webhook-generic)
- Encriptación at-rest de webhook URLs en `notification_subscriptions.recipient` (vía issue-02.3 secrets)

**No incluye:**
- Slack OAuth app (futuro, requiere app marketplace)
- Discord/Teams (canales separados futuros)

## Enfoque técnico

1. HTTP client compartido con timeout 30s y retry transparente para 5xx
2. Rate limiter token bucket por hash(recipient)
3. HMAC con header `X-Domain-Signature: sha256=<hex>` y `X-Domain-Timestamp`
4. Webhook URLs tratadas como secret: log redactado

## Riesgos

- Webhook URL leak en logs → redactor de URL
- DoS si webhook target lento → timeout estricto
- Re-entrada: webhook que apunta al propio Domain → max depth + dominio block list opcional

## Testing

- httptest server validando body Block Kit
- Rate limit: 10 enqueue → 10 sent respetando 1/s
- HMAC verificable
- URL no https para generic → fail-fast
