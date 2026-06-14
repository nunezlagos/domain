# Proposal: issue-10.2-webhook-triggers

## Intención

Sistema de recepción de webhooks HTTP POST con validación de autenticidad (HMAC-SHA256 para GitHub, token para GitLab), mapeo de payloads específicos por fuente (GitHub, GitLab, generic), ejecución de flows/agentes, y delivery logs con capacidad de replay.

## Scope

**Incluye:**
- Modelo `Webhook` con fields: name, project_id, flow_slug, agent_slug, secret (bcrypt hash), source (github|gitlab|generic), events ([]string), enabled
- Endpoint público POST /api/v1/webhooks/receive/:id (sin auth, validación vía HMAC/token)
- Validador HMAC-SHA256 para GitHub
- Validador de token para GitLab
- Mappers de payload: GitHub → normalized, GitLab → normalized, generic → raw
- Delivery logs: tabla `webhook_deliveries`
- Replay de deliveries fallidas
- Timeout de 5s para validación + ejecución
- Límite de payload de 1MB

**Excluye:**
- Rate limiting por webhook (se hace a nivel de API gateway o middleware global)
- IP whitelisting (futuro)
- Webhook UI para testear (se puede hacer via curl)
- Firma de respuesta (el webhook receptor no firma responses)

## Enfoque técnico

- Endpoint público (sin auth middleware) que recibe POST en /api/v1/webhooks/receive/:id
- Lookup del webhook por ID, verificar enabled
- Según source:
  - github: verificar header X-Hub-Signature-256 con HMAC-SHA256(secret, body)
  - gitlab: verificar header X-Gitlab-Token contra secret (comparación constante-time)
  - generic: sin validación si no hay secret
- Verificar que el evento (header X-GitHub-Event o X-Gitlab-Event) esté en la lista de eventos suscritos
- Mapper transforma payload source-specific a formato normalizado interno
- Ejecutar flow o agente con payload mapeado como input
- Registrar delivery log con status, duración, error, ids
- Replay: re-envía el mismo payload al mismo webhook

## Riesgos

- Tiempo de validación vs ataque timing: usar `hmac.Equal` para comparación constante-time
- Payloads grandes: límite de 1MB en el body
- Secret en texto plano: almacenar bcrypt hash del secret, comparar con el hash
- Replay sin idempotencia: el flow debe ser idempotente o se pueden crear duplicados

## Testing

- Unit: validación HMAC-SHA256 correcta e incorrecta
- Unit: validación GitLab token (constante-time)
- Unit: mapeo de payload GitHub→normalized
- Unit: mapeo de payload GitLab→normalized
- Unit: mapeo de payload generic→normalized
- Unit: evento no suscrito → skip
- Integration: receiver endpoint completo
- Integration: delivery logs
- Integration: replay
- Sabotaje: HMAC sin comparación constante-time → test de timing no ataca pero verificamos uso de hmac.Equal
