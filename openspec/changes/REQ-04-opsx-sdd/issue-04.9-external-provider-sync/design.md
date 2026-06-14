# Design: issue-04.9-external-provider-sync

## Decisión arquitectónica

**Domain es source of truth**. External provider (Jira/GH/Linear) es mirror. Para campos editables (title, description, AC) Domain es autoridad y push es la dirección normal; pull es **solo detección de drift** (no overwrite). Para campos derivados (status workflow, comments), pull es aceptable porque reflejan acción del equipo que vive en el provider.

**Driver pattern**. Cada provider implementa `Driver` interface. Lógica común (queue, retry, audit, drift detection) en core; particularidades de cada API (ADF para Jira, GitHub Markdown, Linear GQL) en su driver.

**Worker queue async**. Push no bloquea el caller (MCP tool devuelve `sync_state_id` y el trabajo se hace en worker). Excepción: el primer push manual desde MCP puede ser sync si el caller lo pide (`wait: true`).

**Retry con backoff**. Errors transitorios (429, 500, network) → backoff exponencial 30s/2m/10m/1h/4h. Tras 5 → DLQ + alerta.

**Webhook receive central**. `/webhooks/providers/:provider` único endpoint con HMAC verification compartido. Driver-specific parsing en `HandleWebhook(payload)`.

## Componentes

```
internal/sdd/sync/
  service.go              # Service: Push, Resolve, Retry, ProviderHealth, ProviderReauth
  queue.go                # worker pool con LISTEN/NOTIFY sync_queue
  worker.go               # process sync ops with retry
  drift.go                # detector field-by-field
  resolver.go             # accept_external / keep_local / manual_merge
  webhook.go              # router /webhooks/providers/:provider → driver.HandleWebhook
  events.go               # publish sync.pushed, sync.drift_detected, sync.resolved, sync.deleted_remote
  driver/
    driver.go             # interface Driver + types compartidos
    jira/
      driver.go           # implementation
      adf.go              # markdown→ADF converter (con media nodes)
      mapping.go          # field mapping
      transitions.go      # status workflow mapping
      attachments.go      # secuencial 1..N upload + media id binding
      webhook.go          # parse changelog
      ratelimit.go        # respect Retry-After + 429 logic

internal/store/pg/sync/
  store.go                # CRUD + index queries (pending retry, drift, etc.)

internal/mcp/tools/sync/
  push.go, resolve.go, retry.go, health.go, reauth.go, drift_list.go, get_state.go
```

## Push flow (Jira example)

```
1. service.Push(req=R) →
   - Lee req del store
   - lookup provider_config
   - encolar op {kind: PushReq, sync_state_id?} → LISTEN/NOTIFY sync_queue
   - returns sync_state_id

2. worker recibe →
   - lock row CAS (next_retry_at < now)
   - driver.PushReq(cfg, req)
     - construct ADF from req.description_md
     - POST /rest/api/3/issue {fields: {project, issuetype, summary, description}}
     - if 429: read Retry-After, schedule retry, return error transient
     - if 401: mark auth_error, alert admin, no retry hasta reauth
     - if 200: parse {key, id, self}
   - persist external_sync_state row
   - update field_mapping.last_pushed_values
   - emit event sync.pushed
   - audit log entry

3. Si es PushHU con attachments →
   - despues del POST issue OK
   - foreach attachment (sorted by filename):
     - driver.UploadAttachment → POST /issue/{key}/attachments multipart
     - capture attachment.id como media_id
     - small delay (50ms) entre uploads para no pegar 429
   - PUT /issue/{key} {fields: {description: ADF_with_media_nodes}}
   - si attachment falla → no abort, log + mark sync_status=partial
```

## Drift detection

Cuando llega webhook `jira:issue_updated` con `changelog`:

```python
for item in changelog.items:
    if item.field in MONITORED_FIELDS:  # ['summary', 'description', 'customfield_acceptance_criteria']
        last_pushed = sync_state.field_mapping.last_pushed_values[map[item.field]]
        if item.toString != last_pushed:
            mark drift, set sync_status=conflict
            drift_fields[item.field] = {jira_value: item.toString, last_pushed: last_pushed, changed_by: webhook.user}
            emit sync.drift_detected
            notify owner (REQ-20)
```

**Importante**: si el cambio fue **causado por Domain** (push reciente <60s), el webhook se ignora (idempotencia). Esto se logra comparando `sync_state.last_synced_at` con `webhook.created_at`.

## Conflict resolution

3 modos:

- `accept_external`: copio Jira → Domain. Update `issues.{title,description,gherkin}` con valores Jira. `last_pushed_values` actualizado. `sync_status=ok`.
- `keep_local_force_push`: push Domain → Jira con PUT, ignorando que hay drift. Marca `last_pushed_values` con valores nuevos.
- `manual_merge`: humano provee `{title, description, gherkin}` finales. Update Domain Y push a Jira.

## Field mapping configurable

Cada org tiene su `provider_config.config` con custom field IDs específicos:

```json
{
  "url": "https://saargo.atlassian.net",
  "project_key": "DIDE",
  "default_issuetype_req": "Epic",
  "default_issuetype_hu": "Story",
  "default_issuetype_subtask": "Sub-task",
  "custom_fields": {
    "acceptance_criteria": "customfield_10042",
    "story_points": "customfield_10016",
    "epic_link": "customfield_10014"
  },
  "epic_link_method": "parent",
  "default_labels": ["sdd", "domain-managed"],
  "status_mapping": {
    "approved": "To Do",
    "in_progress": "In Progress",
    "done": "Done",
    "rejected": "Won't Do"
  },
  "monitored_fields_for_drift": ["summary", "description", "customfield_10042"]
}
```

Esto permite que la misma instancia Domain orqueste contra Jiras de clientes distintos con campos custom distintos.

## Secret management

`provider_configs.credentials_ref` apunta a `secrets` table (REQ-02.3). Domain encripta el token con KMS-derived key. Nunca se loguea el raw token. Rotación vía `domain_sync_provider_reauth`.

## Rate limit handling

- Tracker per provider_config: `rate_limit_remaining`, `reset_at`.
- Driver Jira respeta header `X-RateLimit-Remaining`.
- Si remaining < 10 → worker queue pausa pushes contra este provider hasta `reset_at`.
- 429 con `Retry-After` → schedule `next_retry_at = now + Retry-After`.

## Idempotencia push

Antes de cada PushReq/PushHU, worker chequea si ya existe `external_sync_state` con `entity_id + provider_config_id`. Si existe y sync_status=ok → update no create. Si existe y sync_status=deleted_remote → crear de nuevo si flag `re_create=true`.

## Webhook security

- HMAC SHA-256 sobre body, header `X-Atlassian-Webhook-Identifier` o similar según provider (REQ-10.2 normaliza esto).
- Secret distinto por `provider_config.config.webhook_secret`.
- IP allowlist opcional (CIDRs de Atlassian/GitHub bien conocidos).
- Replay protection: `webhook_id` único cacheado 1h.

## Comments sync (out-of-scope MVP, declarado)

- Pull comments → tabla `activity` con `source=jira` + `external_comment_id`.
- Push comments desde Domain → opcional flag `push_comments_to_provider`. Default false.
- Bidirectional comments es complicado (auto-reply loops) → diferido.

## Trade-offs

| Decisión | Trade-off |
|---|---|
| Push async siempre | Caller no sabe si Jira recibió hasta esperar evento. Trade: respuesta rápida + workers paralelos. |
| Drift solo en campos monitoreados | Otros campos (assignee, sprint) pueden cambiar en Jira y no enteramos. Aceptable: esos son operacionales, no contenido SDD. |
| Status pull-overwrite (sin drift) | Si humano marcó Done en Jira por error, Domain refleja Done. Acepta esto en favor de flujo natural; admin puede revertir manualmente. |
| Sin push de comments en MVP | Audit lopsided (comments locales no llegan a Jira). Aceptable para MVP, soluciona en HU futura. |
| Driver Jira-first | GitHub/Linear esperan implementación. Trade: foco MVP + interface clara. |

## Observability

- `sync_push_total{provider, entity_kind, result}`
- `sync_push_duration_seconds{provider, entity_kind}`
- `sync_drift_detected_total{provider, field}`
- `sync_queue_depth{provider}`
- `sync_dlq_size{provider}`
- `sync_provider_health{provider} gauge 1/0`
- `sync_rate_limit_remaining{provider}`

## Testing strategy

- Unit: drift detection con todos los casos (no drift, drift simple, drift multifield, drift conflictivo con last_synced reciente).
- Integration: testcontainers + WireMock de Jira → push/pull end-to-end.
- Sabotaje: 429 con Retry-After respetado; 401 marca auth_error; 5 fallos → DLQ; webhook replay no duplica state.
