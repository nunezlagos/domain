# Proposal: HU-04.9-external-provider-sync

## Intención

Sincronización Domain ↔ proveedor externo de issue tracking (Jira Cloud en MVP) con dirección clara: Domain es source of truth, provider es mirror. Detección de drift cuando el contenido SDD (title/description/AC) es editado en el provider y resolución manual asistida. Multi-driver future-ready con interfaz `Driver` única.

## Scope

**Incluye:**
- Tablas `provider_configs` y `external_sync_state` + `external_sync_events` (audit)
- Driver Jira Cloud completo: push REQ→Epic, push HU→Story, push attachments 1..N, pull webhook (status + comments + drift detection)
- Worker queue con LISTEN/NOTIFY + backoff retry + DLQ
- Conflict resolution con 3 modos (accept_external, keep_local_force_push, manual_merge)
- Field mapping configurable por org (custom_field_acceptance_criteria, etc.)
- Status mapping local→remote (approved→To Do, in_progress→In Progress, done→Done)
- Rate limit awareness (429 con Retry-After respect)
- Healthcheck periódico del driver + reauth tool
- Webhook endpoint con HMAC verification (delega a REQ-10.2)
- 7 MCP tools (push, get_state, resolve_conflict, retry, provider_health, provider_reauth, drift_list)
- Idempotencia push (entity_id + provider_config_id unique)
- Métricas Prometheus + tracing OTel + audit log entries por operación
- Secret management vía REQ-02.3 (credentials_ref, never raw token)
- Multi-tenant scope estricto

**No incluye:**
- Drivers GitHub Issues, Linear, Asana, GitLab Issues (HUs futuras 04.9b/c/d/e implementando misma interface)
- Push de comments desde Domain a Jira (pull-only para comments en MVP)
- Bidirectional bulk operations (replay full history)
- Web UI de conflict resolution (parte de REQ-16)
- Subtasks TDD push automático (las sub-tasks de Red/Green/Refactor se gestionan en otra HU)
- Sync de sprints, fix versions, components específicos de Jira (out-of-scope SDD)
- Auto-resolution sin humano (siempre humano confirma drift resolution)

## Alternativas consideradas

### A. Bidirectional last-write-wins (sin drift detection)

**Por qué no:** edits silenciosas pueden pisar trabajo. Sin auditoría confiable de "qué tenía Domain antes" → imposible reconstruir el por qué de un cambio. Drift detection + resolution manual es solo marginalmente más caro y mucho más seguro.

### B. Jira como source of truth (Domain solo lee)

**Por qué no:** se pierde la trazabilidad SDD interna (Gherkin estructurado, mockups en S3 con signed URLs, dedup semántico contra catálogo interno). Domain debe ser autoridad SDD; Jira tiene su lugar como herramienta del cliente.

### C. Push síncrono sin queue

**Por qué no:** una llamada MCP que bloquea 30s mientras pushes 5 attachments es mala UX. Async + sync_state que el caller puede pollear es estándar.

### D. Driver Jira hardcoded en core (sin interface)

**Por qué no:** GitHub Issues / Linear / Asana son demandas conocidas. Interface ahora vs refactor masivo después.

### E. Pull periodico cada N min en lugar de webhook

**Por qué no:** webhook es near-real-time, pull periodico introduce latencia + carga innecesaria. Pull se mantiene como fallback (HU futura) si webhook no disponible.

### F. Sin healthcheck (asumir provider OK)

**Por qué no:** tokens expiran silenciosamente; permisos cambian; URLs migran. Healthcheck periódico cheap (1 GET cada 5min) detecta antes de que el primer push falle.

### G. Conflict resolution automático con LLM

**Por qué no:** edits SDD son sensibles (un cambio en AC puede invalidar tests). Hasta tener métricas de cuán acertado es el LLM, humano resuelve. HU futura puede agregar "AI-suggested resolution" como preview.

## Dependencias

**Hard:**
- HU-04.1, HU-04.2, HU-04.6
- REQ-02.3 secrets vault (credentials)
- REQ-02.4 audit log
- REQ-10.2 webhook receiver HMAC
- REQ-20 notifications (alertas drift + auth_error)
- REQ-26.5 circuit breaker (no cascading failures contra provider down)

**Soft:**
- REQ-17 observability
- HU-04.8 intake-pipeline (intake.committed event → trigger automático push)
- REQ-15 cost observability (track API calls a providers como cost item)

## Plan de release

1. Schema + store + driver interface
2. Driver Jira: push REQ→Epic + push HU→Story (sin attachments)
3. Push attachments 1..N + ADF media nodes
4. Worker queue + backoff retry
5. Webhook receive + status sync (no drift)
6. Drift detection + sync_status=conflict
7. Conflict resolution tools
8. Healthcheck + reauth
9. Métricas + traces
10. Tests integración con WireMock + sabotaje

## Riesgos

| Riesgo | Mitigación |
|---|---|
| ADF complejo, edge cases | Tests con corpus de markdowns reales; fallback a code block plain si conversión falla |
| Rate limit Jira agresivo | Respetar Retry-After estrictamente; soft pause si remaining < 10 |
| Drift falso positivo (cambio cosmético) | Normalizar whitespace/HTML antes de comparar; allowlist de fields monitoreados |
| Webhook lost (network) | Pull manual fallback con `domain_sync_reconcile({sync_state_id})` |
| Tokens expirados silenciosamente | Healthcheck periódico + reauth tool |
| Comments loop (push+pull → pull marca como external → push de nuevo) | No-push comments en MVP; cuando se agregue, idempotency_key por comment + window de 60s |
| Mismatch en organization_id (cross-tenant leak) | RBAC enforcement tests + assertion en cada operación |
| Custom field IDs cambian | provider_config snapshot al push; alerta al admin si discovery encuentra ID nuevo |

## Tests críticos

- E2E push: REQ con HU + 3 attachments → Jira muestra Epic + Story con media inline ordenadas
- E2E pull status: Jira To Do→In Progress→Done refleja en user_stories.status
- E2E drift: edit summary en Jira → sync_status=conflict + notif
- E2E resolve accept_external: campos copiados a Domain, sync_status=ok
- E2E resolve keep_local: push a Jira sobrescribe el drift
- Sabotaje 429: respeta Retry-After, no spam
- Sabotaje 401: marca auth_error, no reintenta, alerta admin
- Sabotaje 5 fallos → DLQ + alerta
- Sabotaje webhook replay: idempotente por webhook_id
- Sabotaje 2 humanos editan mismo HU en Domain + en Jira en paralelo: detecta como conflict
- Concurrency: 100 pushes paralelos → 0 race conditions en CAS
- Cross-tenant: org A intenta push con provider de org B → 403
