# Proposal: HU-10.4-outbound-webhooks

## Intención

Sistema de webhook subscriptions outbound (Domain → sistemas externos): subscribe a events, HMAC signing, retry con DLQ, circuit breaker, filters, replay y test.

## Scope

**Incluye:**
- Tabla `outbound_webhook_subscriptions`
- Tabla `outbound_webhook_deliveries` con status, retries, response
- Event bus interno publica a dispatcher
- HMAC SHA-256 signing
- Retry policy con backoff exponencial 8 attempts
- DLQ + admin notification
- Circuit breaker auto-pause
- Filters por payload (JSONPath-like)
- Endpoint test ping
- SSRF prevention (block private IPs, internal hostnames)

**No incluye:**
- Inbound webhooks (HU-10.2)
- Webhook signing rotation múltiples secrets (futuro)

## Enfoque técnico

1. Event publisher hooks en services (agent.complete, flow.complete, etc.)
2. Dispatcher worker poll deliveries pending
3. HTTP client con timeout 30s + redirect block
4. URL validator: no rfc1918 IPs en prod
5. Circuit breaker: tracker por subscription rolling 1h

## Riesgos

- SSRF: validator + outbound proxy con allowlist en prod
- Thundering herd: jitter en retry backoff
- Replay: timestamp + ventana 5min en signature

## Testing

- Subscribe + event → delivery con HMAC
- Retry 503 → backoff 8x → DLQ
- Filter matchea/no matchea
- Test ping
- SSRF intento blocked
- Circuit breaker auto-pause
