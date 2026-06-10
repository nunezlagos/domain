# Tasks: issue-10.2-webhook-triggers

## Backend

- [ ] Crear modelo `Webhook` en `internal/models/webhook.go`
- [ ] Crear migración SQL para tabla `webhooks`
- [ ] Crear migración SQL para tabla `webhook_deliveries`
- [ ] Implementar `WebhookRepository` con Create, GetByID, GetByProjectID, Update, Delete
- [ ] Implementar validador HMAC-SHA256 (GitHub)
- [ ] Implementar validador GitLab token (constant-time compare)
- [ ] Implementar mapper GitHub → normalized (push, pull_request, issues)
- [ ] Implementar mapper GitLab → normalized
- [ ] Implementar mapper generic → normalized (raw passthrough)
- [ ] Implementar handler público POST /api/v1/webhooks/receive/:id
- [ ] Implementar límite de payload (MaxBytesReader 1MB)
- [ ] Implementar verificación de evento suscrito
- [ ] Implementar ejecución de flow/agente desde webhook
- [ ] Implementar delivery logging (insert en webhook_deliveries)
- [ ] Implementar replay de delivery fallida
- [ ] Crear handler REST: CRUD /api/v1/webhooks
- [ ] Crear handler REST: GET /api/v1/webhooks/:id/deliveries
- [ ] Crear handler REST: POST /api/v1/webhooks/deliveries/:id/replay
- [ ] Almacenar secret como bcrypt hash, nunca devolverlo en GET

## Tests

- [ ] Test unitario: HMAC-SHA256 validación correcta
- [ ] Test unitario: HMAC-SHA256 validación incorrecta
- [ ] Test unitario: GitLab token validación correcta e incorrecta
- [ ] Test unitario: GitHub push mapper
- [ ] Test unitario: GitHub pull_request mapper
- [ ] Test unitario: GitHub issues mapper
- [ ] Test unitario: GitLab push mapper
- [ ] Test unitario: generic mapper (raw)
- [ ] Test unitario: evento suscrito vs no suscrito
- [ ] Test de integración: receiver endpoint con firma válida → flow ejecutado
- [ ] Test de integración: receiver con firma inválida → 401
- [ ] Test de integración: delivery log creado correctamente
- [ ] Test de integración: replay de delivery
- [ ] Test de integración: payload > 1MB rechazado
- [ ] Sabotaje: quitar validación HMAC → test de firma inválida falla

## Cierre

- [ ] Verificación manual: curl con HMAC correcto → 200 + flow ejecutado
- [ ] Verificación manual: delivery log visible via GET
- [ ] Suite verde
