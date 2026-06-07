# HU-20.1-channel-abstraction

**Origen:** `REQ-20-notifications`
**Persona:** platform-engineer, dx-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador
**Quiero** una interfaz `NotificationChannel` con registry, templates y delivery logs
**Para** que cualquier feature (alertas, invitaciones, runs fallidos) use canales de forma uniforme

## Criterios de aceptación

### Escenario 1: Interfaz y registry

```gherkin
Dado que existe `internal/notifications/channel.go`
Cuando inspecciono la interfaz
Entonces es:
  ```go
  type Channel interface {
      Slug() string
      Send(ctx context.Context, msg Message) (DeliveryID, error)
  }
  ```
Y existe registry singleton `Register(ch Channel)` y `Get(slug)` thread-safe
```

### Escenario 2: Templates con variables

```gherkin
Dado que existe template `usage_alert` con body:
"Tu organización {{.OrgName}} alcanzó {{.Percent}}% del límite de tokens este mes"
Cuando hago `tmpl.Render(map[string]any{"OrgName": "Acme", "Percent": 80})`
Entonces el output es "Tu organización Acme alcanzó 80% del límite de tokens este mes"
Y variables faltantes producen error explícito (no `<no value>`)
```

### Escenario 3: Delivery logs

```gherkin
Dado que un canal envía un mensaje
Cuando termina (éxito o fallo)
Entonces se inserta en tabla `notification_deliveries`:
  | id | channel_slug | recipient | template | status | latency_ms | response_code | error | attempt | created_at |
Y `status` es uno de: sent, failed, retrying, dead
```

### Escenario 4: Retry con backoff

```gherkin
Dado que un canal falla con error transitorio (timeout, 5xx)
Cuando se cumple política `max_attempts=5, backoff=exp(base=2s, max=5min)`
Entonces se reintenta hasta max attempts
Y después se marca dead y se notifica al canal admin
Y errores 4xx no transitorios no se reintentan
```

### Escenario 5: Routing por evento

```gherkin
Dado que existe tabla `notification_subscriptions` con (event_type, channel_slug, recipient, org_id, user_id)
Cuando ocurre evento `usage_alert.exceeded`
Entonces se envía a cada subscriber con el canal preferido
Y respeta la preferencia opt-out del usuario
```

## Análisis breve

- **Qué pide:** interface limpia + registry + templates Go templates + tabla deliveries + retry worker + tabla subscriptions
- **Esfuerzo:** M
- **Riesgos:** spam por loop de notificaciones; PII en payloads
