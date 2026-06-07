# HU-20.3-slack-webhook

**Origen:** `REQ-20-notifications`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** equipo ops
**Quiero** un canal Slack que postee a Incoming Webhook URLs usando Block Kit
**Para** recibir alertas (runs fallidos, usage > 80%, drill failures) en el canal del equipo

## Criterios de aceptación

### Escenario 1: Post simple a webhook URL

```gherkin
Dado que existe subscription con `channel_slug="slack-webhook"` y `recipient="https://hooks.slack.com/services/T0/B0/XXX"`
Cuando se envía mensaje "Run abc-123 failed"
Entonces se hace POST a la URL con body JSON {"text": "..."}
Y la respuesta 200 OK marca status=sent
Y respuesta 4xx no se reintenta
Y respuesta 5xx o timeout se reintenta
```

### Escenario 2: Block Kit message

```gherkin
Dado que template usa Block Kit blocks:
  [{"type":"header","text":{"type":"plain_text","text":"Run failed"}},
   {"type":"section","fields":[{"type":"mrkdwn","text":"*Run:* {{.RunID}}"}]}]
Cuando se renderiza y envía
Entonces el JSON enviado tiene "blocks" array válido según API Slack
```

### Escenario 3: Generic webhook variant

```gherkin
Dado que existe canal `webhook-generic`
Cuando se envía mensaje
Entonces se hace POST con headers configurables (auth, content-type)
Y body es JSON o plain según template
Y HMAC-SHA256 opcional con secret en header X-Domain-Signature
```

### Escenario 4: Rate limit Slack 1 msg/s/channel

```gherkin
Dado que se enqueuean 10 mensajes al mismo recipient en 1s
Cuando el worker los procesa
Entonces respeta rate limit Slack (1 msg/s por canal por workspace)
Y throttle interno reordena sin perder mensajes
```

## Análisis breve

- **Qué pide:** dos canales hermanos: slack-webhook y webhook-generic
- **Esfuerzo:** S
- **Riesgos:** webhook URLs son secrets; manejar como tales en `notification_subscriptions.recipient`
