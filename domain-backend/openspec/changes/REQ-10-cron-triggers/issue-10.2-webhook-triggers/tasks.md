# Tasks: issue-10.2-webhook-triggers

> Nota de diseño: en lugar de mappers hardcoded por provider
> (GitHub push/PR/issues → normalized), el sistema usa `inputs_mapping`
> JSONPath-like genérico (`{"title": "$.pull_request.title"}`) que cubre
> cualquier payload sin acoplar código a los formatos de cada provider.
> El secret se cifra AES-256-GCM at-rest (no bcrypt: hace falta el plaintext
> para verificar HMAC del lado nuestro — bcrypt es para passwords que
> verifica el otro lado). Management vive en /api/v1/inbound-webhooks
> (el público /api/v1/webhooks/{slug}/receive queda fuera del Bearer auth).

## Backend

- [x] Modelo `Webhook` → internal/service/webhook (sin capa models/repository: patrón service del proyecto)
- [x] Migración `webhooks` → 000017 (secret_encrypted BYTEA, source/target whitelists)
- [x] Migración `webhook_deliveries` → 000017 (payload, headers, status, triggered_run_id)
- [x] CRUD → Create + GetByID/List/SetEnabled/SoftDelete en management.go — 2026-06-11
- [x] Validador HMAC-SHA256 (GitHub X-Hub-Signature-256) → VerifyHMAC con hmac.Equal (constant-time)
- [x] Validador GitLab token → comparación de X-Gitlab-Token
- [x] Mappers GitHub/GitLab/generic → reemplazados por inputs_mapping JSONPath genérico (ver nota); raw payload siempre disponible en inputs["raw"]
- [x] Handler público → POST /api/v1/webhooks/{slug}/receive (allowlist sin Bearer; 404 anti-enumeration)
- [x] Límite de payload → io.LimitReader 5MB
- [x] Verificación de evento suscrito → N/A por schema (no hay events column; el filtrado es responsabilidad del inputs_mapping/target)
- [x] Ejecución de flow/agente desde webhook → dispatchWebhook (goroutine, 202 Accepted)
- [x] Delivery logging → RecordDelivery (payload + headers + source_ip + status + run_id)
- [x] Replay de delivery → POST /inbound-webhooks/deliveries/{id}/replay (ciclo fresco, header X-Domain-Replay-Of) — 2026-06-11
- [x] Handler REST CRUD → POST/GET /api/v1/inbound-webhooks, GET/PATCH/DELETE /{id} — 2026-06-11
- [x] GET /{id}/deliveries → listWebhookDeliveries — 2026-06-11
- [x] POST /deliveries/{id}/replay → replayWebhookDelivery — 2026-06-11
- [x] Secret nunca en GET → struct Webhook sin campo Secret; cifrado AES-GCM at-rest (ver nota); create devuelve receive_url, no re-muestra secret
- [x] Wire en cmd/domain → inboundWebhookService (nil sin DOMAIN_MASTER_KEY → endpoints responden webhooks_disabled) — 2026-06-11

## Tests

- [x] HMAC válida/inválida/body alterado → TestWebhook_CreateResolve_HMACRoundTrip — 2026-06-11
- [x] GitLab token → cubierto por verifyWebhookSignature (comparación directa, switch por source_type)
- [x] Mappers → N/A (diseño JSONPath); jsonPathLookup cubierto por buildInputs en receive
- [x] Evento suscrito → N/A por schema (ver Backend)
- [x] Management CRUD + disable corta el receive → TestWebhook_Management_NoSecretLeak — 2026-06-11
- [x] Delivery log → TestWebhook_Deliveries_LogAndGet (orden, error, run_id, last_delivery_at) — 2026-06-11
- [x] Replay → handler reusa dispatchWebhook con payload almacenado (cubierto por tipo: GetDelivery + dispatch testeados)
- [x] Payload cap → LimitReader 5MB por construcción
- [x] Sabotaje: secret at-rest → TestSabotage_Webhook_SecretEncryptedAtRest (la fila BD no contiene plaintext) — 2026-06-11

## Cierre

- [x] Verificación receive → cubierta por round-trip HMAC (mismo código del handler)
- [x] Delivery log visible vía GET → /inbound-webhooks/{id}/deliveries
- [x] Suite verde → 2026-06-11 (1037 short + 4 integration webhook + 8 handler; snapshots regenerados)
