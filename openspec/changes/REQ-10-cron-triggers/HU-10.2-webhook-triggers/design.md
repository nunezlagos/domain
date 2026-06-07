# Design: HU-10.2-webhook-triggers

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Validación GitHub | HMAC-SHA256 con `crypto/hmac` | JWT, API key simple (HMAC-SHA256 es el estándar de GitHub) |
| Validación GitLab | Token header con `subtle.ConstantTimeCompare` | HMAC (GitLab usa token directo, no HMAC) |
| Almacenamiento de secret | bcrypt hash | Texto plano encriptado con AES (bcrypt es suficiente, no necesitamos recuperar el original) |
| Mapeo de payload | Función por source en map[Source]MapperFn | Switch (map extensible sin modificar el core) |
| Payload size limit | Middleware `http.MaxBytesReader` | Validación manual (middleware es estándar Go) |

## Alternativas descartadas

- **JWT para validación**: GitHub no usa JWT, usa HMAC-SHA256 del body. No podemos cambiar el protocolo.
- **AES encryption del secret**: No necesitamos el secret original, solo validar. bcrypt hash es suficiente y más seguro.
- **Mapper universal**: Cada source tiene estructura diferente; un mapper por source es más mantenible.

## Diagrama

```
POST /api/v1/webhooks/receive/{webhook_id}
         │
         ▼
┌──────────────────┐
│ MaxBytesReader   │
│ (límite 1MB)     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Lookup webhook   │
│ por ID           │
└────────┬─────────┘
         │
    ┌────┴────┐
    │ ¿enabled?│── no ──► 404
    └────┬────┘
         │ sí
         ▼
┌──────────────────┐
│ Validar según    │
│ source:          │
│                  │
│ GitHub:          │
│  X-Hub-Sig-256   │
│  = HMAC-SHA256   │
│                  │
│ GitLab:          │
│  X-Gitlab-Token  │
│  = secret        │
│                  │
│ Generic:         │
│  sin validación  │
└────────┬─────────┘
         │
    ┌────┴────┐
    │ ¿válido? │── no ──► 401 + log "signature_failed"
    └────┬────┘
         │ sí
         ▼
┌──────────────────┐
│ ¿evento en       │
│ lista suscrita?  │── no ──► 200 + "event_not_subscribed"
└────────┬─────────┘
         │ sí
         ▼
┌──────────────────┐
│ Mapear payload   │
│ según source     │
│ → formato interno│
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Ejecutar flow/   │
│ agente con input │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Registrar        │
│ delivery log     │
└──────────────────┘
```

Normalized internal payload format:
```json
{
  "source": "github|gitlab|generic",
  "event": "push|pull_request|issues|...",
  "payload": { /* original o mapeado */ },
  "headers": { /* headers relevantes */ },
  "normalized": {
    "ref": "refs/heads/main",
    "repository": {"name": "myrepo", "full_name": "user/myrepo"},
    "sender": {"login": "user", "id": 123},
    "commits": [{"id": "abc", "message": "fix"}],
    "action": "opened|synchronize|closed",  // PR events
    "issue": {"number": 1, "title": "Bug"}  // issues events
  }
}
```

Modelo `webhooks`:
```sql
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id),
    flow_slug VARCHAR(255),
    agent_slug VARCHAR(255),
    secret_hash VARCHAR(255),  -- bcrypt hash, nullable para generic sin auth
    source VARCHAR(50) NOT NULL CHECK (source IN ('github', 'gitlab', 'generic')),
    events TEXT[] NOT NULL DEFAULT '{}',  -- vacío = todos los eventos
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event VARCHAR(100),
    status VARCHAR(50) NOT NULL,
    source_ip VARCHAR(45),
    request_headers JSONB,
    request_body BYTEA,  -- para replay
    response_status INT,
    flow_run_id UUID REFERENCES flow_runs(id),
    error TEXT,
    duration_ms INT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## TDD plan

1. **Red:** Test `TestHMACValidation_Valid` — firma correcta pasa
2. **Green:** Implementar HMAC-SHA256 validator
3. **Red:** Test `TestHMACValidation_Invalid` — firma incorrecta rechaza
4. **Green:** hmac.Equal para comparación
5. **Red:** Test `TestGitLabTokenValidation` — token correcto e incorrecto
6. **Green:** subtle.ConstantTimeCompare
7. **Red:** Test `TestGitHubMapper_Push` — push payload normalizado
8. **Green:** Mapper GitHub → normalized
9. **Red:** Test `TestGitLabMapper_Push` — push payload GitLab normalizado
10. **Green:** Mapper GitLab → normalized
11. **Red:** Test `TestWebhookReceiver_FlowExecution` — webhook ejecuta flow
12. **Green:** Receiver handler completo
13. **Red:** Test `TestWebhookReceiver_EventNotSubscribed` — skip
14. **Green:** Verificar evento en lista
15. **Red:** Test `TestDeliveryLogging` — delivery se registra
16. **Green:** Insert en webhook_deliveries
17. **Red:** Test `TestReplayDelivery` — replay re-ejecuta
18. **Green:** Re-enviar mismo payload
19. **Sabotaje:** Quitar validación HMAC → test de firma inválida no rechaza

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Timing attack en HMAC | Baja | Alto | Usar hmac.Equal (constant-time) |
| Payload replay malicioso | Media | Medio | Idempotencia en flows; el delivery log permite detectar duplicados |
| Secret leak en DB | Baja | Alto | bcrypt hash, no texto plano |
| Webhook flooding | Media | Alto | Rate limiting middleware global (50 req/s por IP configurable) |
| Payload muy grande | Baja | Medio | MaxBytesReader 1MB |
