# HU-10.4-outbound-webhooks

**Origen:** `REQ-10-cron-triggers`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer integrando Domain con otros sistemas
**Quiero** suscribir endpoints HTTP a eventos de Domain (run.completed, observation.created, etc.)
**Para** reaccionar en tiempo real sin polling

## Criterios de aceptación

### Escenario 1: Suscripción a evento

```gherkin
Dado que existen event types: `agent_run.completed`, `agent_run.failed`, `flow_run.completed`, `observation.created`, `invitation.accepted`, etc.
Cuando POST /api/v1/outbound-webhooks con
  ```json
  {
    "name":"ci-trigger",
    "url":"https://hooks.example.com/...",
    "events":["agent_run.completed","flow_run.completed"],
    "filters":{"project_id":"X"},
    "secret":"whsec_..."
  }
  ```
Entonces se crea subscription
Y `secret` se cifra at-rest (HU-02.3)
```

### Escenario 2: Delivery con HMAC

```gherkin
Dado que ocurre event matcheando subscription
Cuando el dispatcher procesa
Entonces hace POST a la URL con:
  - body JSON: `{event_type, event_id, occurred_at, data:{...}}`
  - headers:
    - `Content-Type: application/json`
    - `X-Domain-Event: agent_run.completed`
    - `X-Domain-Delivery-Id: <UUID>`
    - `X-Domain-Timestamp: <unix>`
    - `X-Domain-Signature: sha256=<hex(hmac_sha256(secret, timestamp+"."+body))>`
Y se logea delivery con response code + latency
```

### Escenario 3: Retry exponential + DLQ

```gherkin
Dado que el endpoint devuelve 503 o timeout
Cuando dispatcher procesa
Entonces se reintenta con backoff [10s, 1m, 5m, 30m, 2h, 6h, 12h, 24h]
Y después del 8vo intento → DLQ
Y se notifica admin
Y endpoint con 5xx >50% en 1h → auto-pause subscription (circuit breaker)
```

### Escenario 4: Replay manual

```gherkin
Dado que existe delivery en DLQ o failed
Cuando POST /api/v1/outbound-webhooks/deliveries/:id/replay
Entonces se reintenta con misma payload + nuevo timestamp + signature
```

### Escenario 5: Filtros por payload

```gherkin
Dado que subscription tiene filters `{event.data.status:"completed", event.data.cost_usd:{gt:1.0}}`
Cuando llega event que no matchea filter
Entonces NO se dispatcha
```

### Escenario 6: Test endpoint

```gherkin
Dado que admin quiere validar URL
Cuando POST /api/v1/outbound-webhooks/:id/test
Entonces envía evento `webhook.test_ping` 
Y devuelve response inline (status code, body excerpt)
```

## Análisis breve

- **Qué pide:** subscriptions table + dispatcher worker + HMAC signing + retry + DLQ + circuit breaker + filters + test endpoint
- **Esfuerzo:** L
- **Riesgos:** thundering herd; SSRF si URL no validada; replay attacks
