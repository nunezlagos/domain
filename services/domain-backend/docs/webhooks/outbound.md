# Outbound Webhooks

> issue-10.4 — `internal/service/outboundwebhook/`

Domain entrega eventos de dominio a endpoints HTTP externos con firma HMAC,
reintentos con backoff, circuit breaker y replay manual.

## Suscribirse

```
POST /api/v1/outbound-webhooks
{
  "name": "ci-notifier",
  "url": "https://hooks.example.com/domain",
  "events": ["flow_run.completed", "agent_run.failed"],
  "filters": {"flow_slug": "deploy"},
  "secret": "whsec_..."
}
```

- Eventos soportados: `agent_run.completed|failed`, `flow_run.completed|failed`,
  `observation.created`, `invite.created`, `invitation.accepted`,
  `usage.alert_fired`, `webhook.test_ping`.
- `filters`: equality match sobre el payload `data`; las keys soportan paths
  anidados con punto (`"flow.slug": "deploy"`, índices de array `"steps.0.id"`).
  Traversal puro de datos — nunca se evalúan expresiones.
- SSRF prevention: se rechazan localhost, `.local/.internal` y rangos
  privados (RFC 1918, link-local). `DOMAIN_OUTBOUND_REQUIRE_TLS=true`
  fuerza https en prod.
- El secret se cifra at-rest (AES-256-GCM, issue-02.3).

## Implementar el receptor

Cada delivery es un POST JSON con headers:

| Header | Contenido |
|--------|-----------|
| `X-Domain-Event` | tipo de evento |
| `X-Domain-Delivery-Id` | UUID único del intento |
| `X-Domain-Timestamp` | unix epoch (anti-replay) |
| `X-Domain-Signature` | `sha256=` + hex(HMAC-SHA256(secret, ts + "." + body)) |

Verificación (pseudocódigo):

```go
mac := hmac.New(sha256.New, secret)
mac.Write([]byte(timestamp + "."))
mac.Write(body)
ok := hmac.Equal(signature, "sha256="+hex(mac.Sum(nil)))
// + rechazar si |now - timestamp| > 5 min (anti-replay)
```

Responder 2xx en <10s. Cualquier otro status o timeout cuenta como fallo.

## Reintentos y dead letter

Backoff: 10s → 1m → 5m → 30m → 2h → 6h → 12h → 24h (8 reintentos).
Agotados → `dead_letter`.

## Circuit breaker

Tras 10 fallos consecutivos, la subscription entra en cooldown de 1h desde
el último fallo: los deliveries se reprograman (`error_message=circuit_open`)
sin gastar intentos ni golpear el endpoint. Pasado el cooldown, los intentos
se reanudan solos (half-open).

## Replay y test

```
POST /api/v1/outbound-webhooks/{id}/test                      → evento test_ping
POST /api/v1/outbound-webhooks/deliveries/{id}/replay         → re-encola (ciclo fresco)
```

## Tests

`dispatcher_integration_test.go`: HMAC verificable end-to-end, filtros
anidados, retry en 503, replay, circuit breaker (endpoint no golpeado).
`dispatcher_test.go`: matriz de filtros + sabotaje no-eval + breaker.
