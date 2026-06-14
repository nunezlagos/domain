# Design: issue-20.3-slack-webhook

## Decisión arquitectónica

**HTTP:** `net/http` con timeout 30s + retry transparent 5xx via worker
**Rate limit:** `golang.org/x/time/rate` token bucket por recipient
**HMAC:** SHA-256 con secret per-subscription
**Block Kit:** validación JSON schema antes de POST

## Componentes

```
internal/notifications/channels/webhook/
  http.go        # shared HTTP client
  slack.go       # SlackChannel implements Channel
  generic.go     # GenericChannel implements Channel
  hmac.go        # signature helper
  ratelimit.go   # per-recipient bucket
  redact.go      # URL redaction for logs
```

## Variables (en subscription)

| field | descripción |
|-------|-------------|
| recipient | URL https del webhook (cifrada en DB) |
| metadata.hmac_secret | secret HMAC opcional (solo generic) |
| metadata.headers | headers custom (solo generic) |

## TDD plan

1. httptest server: POST llega con body esperado
2. Block Kit JSON schema validate
3. Rate limit 10 req/s respeta 1/s
4. HMAC verificable con verify side
5. URL no https en generic → reject
6. Sabotaje: URL apunta a localhost privado → block list opcional
