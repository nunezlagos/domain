# issue-10.2-webhook-triggers

**Origen:** `REQ-10-cron-triggers`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** ingeniero de automatización
**Quiero** recibir webhooks HTTP POST con validación HMAC-SHA256, mapear el payload a inputs de flows/agentes, y tener logs de entrega
**Para** integrar la plataforma con eventos externos (GitHub push, PR, issues; GitLab; servicios vía JSON genérico)

## Criterios de aceptación

### Escenario 1: Registrar un webhook receptor

```gherkin
Dado que soy un usuario autenticado con permiso `webhook:write`
Cuando envío un POST a `/api/v1/webhooks` con:
  """
  {
    "name": "GitHub Push Handler",
    "project_id": "proj-abc-123",
    "flow_slug": "handle-github-push",
    "secret": "whsec_mysecret123",
    "source": "github",
    "events": ["push", "pull_request"],
    "enabled": true
  }
  """
Entonces el sistema responde con HTTP 201
Y el body contiene `id` (UUID)
Y el body contiene `webhook_url` = "https://domain.example.com/api/v1/webhooks/receive/{id}"
Y el secret no se devuelve en la respuesta (solo el hash)
```

### Escenario 2: Webhook recibe POST válido de GitHub

```gherkin
Dado que existe un webhook registrado con source "github" y secret "whsec_mysecret123"
Cuando envío un POST a `/api/v1/webhooks/receive/{id}` con:
  - Header `X-Hub-Signature-256: sha256=<HMAC-SHA256 del body>`
  - Header `X-GitHub-Event: push`
  - Body JSON con payload de GitHub push
Entonces el sistema valida la firma HMAC-SHA256
Y el sistema verifica que el evento "push" está en la lista de eventos suscritos
Y mapea el payload de GitHub a los parámetros de input del flow
Y ejecuta el flow asociado con esos parámetros
Y responde HTTP 200 con `{"accepted": true, "flow_run_id": "run-abc"}`
```

### Escenario 3: Firma HMAC inválida

```gherkin
Dado que existe un webhook con secret "whsec_mysecret123"
Cuando envío un POST a `/api/v1/webhooks/receive/{id}` con firma HMAC incorrecta
Entonces el sistema responde con HTTP 401
Y el body contiene `error: "invalid_signature"`
Y no se ejecuta ningún flow
Y se registra un log de entrega con status "signature_failed"
```

### Escenario 4: Evento no suscrito

```gherkin
Dado que un webhook está suscrito solo a eventos ["push", "pull_request"]
Cuando envío un POST con header `X-GitHub-Event: issues`
Entonces el sistema responde con HTTP 200 (no revelar suscripciones)
Y el body contiene `{"accepted": false, "reason": "event_not_subscribed"}`
Y no se ejecuta el flow
Y se registra un log de entrega con status "skipped"
```

### Escenario 5: Webhook genérico JSON

```gherkin
Dado que existe un webhook con source "generic"
Cuando envío un POST a `/api/v1/webhooks/receive/{id}` con body JSON genérico:
  """
  {
    "event": "order.created",
    "data": {"order_id": "ord-123", "amount": 100}
  }
  """
Y no se requiere firma HMAC (source generic sin secret)
Entonces el sistema responde con HTTP 200
Y ejecuta el flow con el payload completo como input
```

### Escenario 6: GitLab webhook

```gherkin
Dado que existe un webhook con source "gitlab" y secret "gl_secret"
Cuando envío un POST a `/api/v1/webhooks/receive/{id}` con:
  - Header `X-Gitlab-Token: gl_secret`
  - Header `X-Gitlab-Event: Push Hook`
  - Body JSON con payload de GitLab push
Entonces el sistema valida el token
Y mapea el payload de GitLab a un formato normalizado
Y ejecuta el flow asociado
```

### Escenario 7: Delivery logs

```gherkin
Dado que un webhook ha recibido 10 entregas
Cuando envío un GET a `/api/v1/webhooks/{id}/deliveries?limit=5`
Entonces recibo:
  """
  {
    "data": [
      {
        "id": "del-001",
        "event": "push",
        "status": "delivered",
        "flow_run_id": "run-abc",
        "received_at": "...",
        "duration_ms": 1500
      },
      {
        "id": "del-002",
        "event": "push",
        "status": "signature_failed",
        "received_at": "...",
        "error": "HMAC signature mismatch"
      }
    ],
    "pagination": {"total": 10, "limit": 5, "offset": 0}
  }
  """
```

### Escenario 8: Replay de webhook fallido

```gherkin
Dado que existe una delivery con status "failed" y id "del-003"
Cuando envío un POST a `/api/v1/webhooks/deliveries/del-003/replay`
Entonces el sistema re-ejecuta el webhook con el payload original
Y registra una nueva delivery con el resultado
```

## Análisis breve

- **Qué pide realmente:** Receptor de webhooks HTTP con validación de firma (HMAC-SHA256 para GitHub, token para GitLab), mapeo de payloads según source, ejecución de flows/agentes, delivery logs con capacidad de replay.
- **Módulos sospechados:** `internal/webhook/receiver.go`, `internal/webhook/validator.go`, `internal/webhook/mapper.go`, `internal/api/handlers/webhook.go`, `internal/models/webhook.go`
- **Riesgos / dependencias:** Validación criptográfica correcta (HMAC). Timeout en receivers (5s max). Payload grande (>1MB) rechazado.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
