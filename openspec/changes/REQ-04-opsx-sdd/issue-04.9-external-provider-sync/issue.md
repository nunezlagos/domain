# issue-04.9-external-provider-sync

**Origen:** `REQ-04-opsx-sdd`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** PM/desarrollador que crea REQs y HUs en Domain
**Quiero** que Domain mantenga un mirror del REQ/HU en un proveedor externo (Jira en MVP; GitHub Issues y Linear en futuras) con sync push, detección de drift cuando alguien edita en el provider, y resolución manual de conflictos
**Para** que el cliente y los stakeholders vean el progreso en su herramienta habitual (Jira sprints/dashboards), mientras Domain conserva la trazabilidad SDD y la auditoría completa

## Principio rector: Domain es source of truth

Toda edición canónica de `requirements.title/description` y `issues.title/description/gherkin` ocurre en Domain (vía MCP, CLI o Web UI). Jira es mirror: refleja lo que Domain dice. Si alguien edita en Jira:
- Status changes (To Do → In Progress → Done) → **se aceptan** y propagan a Domain como state transitions internas (no son "drift").
- Comentarios → se ingestan a `activity` de Domain como ítems read-only enlazados.
- Title/description/AC → se considera **drift**: Domain marca `sync_status=conflict`, NO sobreescribe, notifica al owner para resolución manual.

## MVP

Driver Jira Cloud únicamente. Drivers GitHub Issues / Linear / Asana son HUs futuras (04.9b, 04.9c, ...) implementando la misma interface `ExternalProviderDriver`.

## Criterios de aceptación

### Escenario 1: Push inicial REQ → Epic en Jira

```gherkin
Dado que existe un REQ en Domain (status="active") sin external_sync_state
Cuando humano invoca `domain_sync_push({entity: "req", entity_id, provider: "jira"})`
Entonces se invoca driver Jira → POST /rest/api/3/issue con issuetype="Epic", title del REQ, description ADF
Y la respuesta {key: "DIDE-100"} se persiste en external_sync_state con sync_direction="push_only", sync_status="ok"
Y Domain devuelve `{external_key: "DIDE-100", external_url}`
```

### Escenario 2: Push HU → Story bajo Epic

```gherkin
Dado que existe un HU en Domain con `req_id` cuyo REQ ya fue pusheado (tiene external_sync_state)
Cuando humano invoca `domain_sync_push({entity: "hu", entity_id, provider: "jira"})`
Entonces se invoca driver Jira → POST /issue con:
  - issuetype="Story"
  - parent={key: external_key del REQ padre}
  - summary = hu.title
  - description = ADF de hu.description_md
  - customfield_acceptance_criteria = hu.gherkin_text
  - labels = ["sdd", req.slug, hu.slug, "severity-{hu.severity}"]
Y se uploadean los attachments (issue-04.6) uno por uno + PUT description con media inline IDs
Y external_sync_state persiste con `field_mapping` JSON que recuerda qué campos fueron pusheados
```

### Escenario 3: Push con bulk de imágenes (1..N)

```gherkin
Dado que la HU tiene N attachments
Cuando push_hu corre
Entonces tras crear el issue, se hacen N calls POST /issue/{key}/attachments con multipart
Y cada response devuelve `attachment_id` que se mapea a `media_id` para el ADF
Y al final se hace UN PUT /issue/{key} con description ADF que incluye los `mediaSingle` nodes con los IDs
Y la subida es secuencial (no paralela) para preservar orden visual
Y si Jira devuelve 429 en alguna call, se aplica backoff exponencial respetando Retry-After
Y si una falla tras 5 reintentos, todo el push se marca `sync_status=partial` y se reporta cuáles attachments faltaron
```

### Escenario 4: Update push (Domain edita, Jira refleja)

```gherkin
Dado que existe HU sincronizada (`sync_status=ok`) y un humano edita el title en Domain
Cuando se persiste el cambio en issues
Entonces se dispara evento `hu.updated`
Y un worker async toma el evento, calcula diff vs `external_sync_state.field_mapping.last_pushed_values`
Y solo si hay diff real → PUT /issue/{key} solo con los campos cambiados (no full replace)
Y actualiza `last_synced_at` + `field_mapping.last_pushed_values`
```

### Escenario 5: Pull webhook Jira → status sync

```gherkin
Dado que Jira está configurado con webhook a Domain `/webhooks/providers/jira`
Cuando una HU cambia de estado en Jira (Transition "To Do" → "In Progress")
Entonces webhook llega con HMAC válido (REQ-10.2)
Y Domain matchea por `external_key` → encuentra el HU local
Y actualiza `issues.status` según mapping:
  - "To Do" → "approved"
  - "In Progress" → "in_progress"
  - "Done" → "done"
  - "Won't Do" / "Rejected" → "rejected"
Y inserta entry en audit_log con `actor="external_sync"`, source_event_id=webhook id
Y NO marca como drift (status sync es esperado)
```

### Escenario 6: Pull webhook Jira → drift detectado

```gherkin
Dado que un humano edita el `summary` del issue DIDE-145 directamente en Jira
Cuando webhook `jira:issue_updated` llega con changelog
Entonces driver detecta cambio en campo monitoreado (summary/description/customfield_AC)
Y compara con `external_sync_state.field_mapping.last_pushed_values.title`
Y si difiere → marca `sync_status="conflict"`, persiste `drift_detected_at`, `drift_fields={summary: {jira_value, last_pushed}}`
Y inserta notificación al owner del HU + al admin de la org
Y NO sobreescribe nada en Domain
Y la HU queda visible como "⚠ Conflicto con Jira" en la UI/CLI
```

### Escenario 7: Resolver drift manualmente

```gherkin
Dado que existe `sync_status=conflict` en un external_sync_state
Cuando humano invoca `domain_sync_resolve_conflict({sync_state_id, resolution: "accept_external"|"keep_local_force_push"|"manual_merge", manual_values?})`
Entonces:
  - "accept_external" → Domain copia los valores de Jira al issues + actualiza last_pushed_values, sync_status="ok"
  - "keep_local_force_push" → push del valor actual de Domain a Jira con PUT, sync_status="ok"
  - "manual_merge" → se aplica `manual_values` a Domain Y a Jira simultáneamente
Y se loguea decisión + actor en audit
```

### Escenario 8: Sync queue + retry con DLQ

```gherkin
Dado que push push falla (Jira 500 o network error)
Cuando worker recibe error
Entonces marca `sync_status="error"`, `retries` incrementado
Y reintentos con backoff exponencial: 30s, 2m, 10m, 1h, 4h
Y tras 5 intentos → mueve a DLQ + alerta al admin
Y `domain_sync_retry({sync_state_id, force?})` permite retry manual
```

### Escenario 9: Borrar remoto desde Domain

```gherkin
Dado que un HU se archiva en Domain (`status="archived"`)
Cuando se dispara evento `hu.archived`
Entonces NO se borra el issue de Jira por defecto (solo se comenta "archivado en Domain DD-MM-YYYY")
Y opcionalmente con flag `auto_transition_on_archive` → transiciona a "Won't Do"
Y external_sync_state.sync_status se mantiene en "ok" pero `archived=true`
```

### Escenario 10: Borrar remoto unilateralmente desde Jira

```gherkin
Dado que un issue es eliminado en Jira (rara vez, pero pasa)
Cuando llega webhook `jira:issue_deleted` o un push subsecuente devuelve 404
Entonces external_sync_state.sync_status="deleted_remote"
Y se notifica al owner: "El issue DIDE-145 fue borrado en Jira, ¿re-crear o archivar local?"
Y opciones de acción: `re_create` (push de nuevo a Jira con key nueva) o `archive_local`
```

### Escenario 11: Multi-driver (Jira + GitHub coexistiendo)

```gherkin
Dado que se implementa driver `github_issue` en HU futura
Cuando un humano hace `domain_sync_push({entity: "hu", entity_id, provider: "github_issue"})`
Entonces se crea un SEGUNDO external_sync_state row para el mismo issue_id pero distinto provider
Y la HU queda mirroreada en ambos lugares simultáneamente
Y push/pull es independiente por driver
```

### Escenario 12: Field mapping configurable por org

```gherkin
Dado que cada org puede tener custom fields distintos en Jira (Acceptance Criteria, Story Points, Sprint)
Cuando se configura el driver Jira para la org
Entonces se persiste en tabla `provider_configs` la receta:
  - jira_url, project_key, default_issuetype_req, default_issuetype_hu
  - custom_field_acceptance_criteria_id (e.g. "customfield_10042")
  - custom_field_story_points_id
  - epic_link_method (parent vs customfield)
  - default_labels
Y el driver lee la config en cada push, no hay valores hardcoded
```

### Escenario 13: Healthcheck del driver

```gherkin
Dado que existe driver Jira configurado
Cuando se invoca `domain_sync_provider_health({provider: "jira"})`
Entonces hace una llamada cheap GET /rest/api/3/myself
Y devuelve `{ok, latency_ms, auth_valid, rate_limit_remaining}`
Y métricas Prometheus se actualizan
```

### Escenario 14: Sabotaje — token API rotó

```gherkin
Dado que el token Jira expira o se revoca
Cuando un push corre y recibe 401
Entonces sync_status="auth_error" en TODOS los rows que comparten ese provider_config
Y se notifica al admin con instrucciones de rotación
Y NO se reintenta hasta que admin confirme con `domain_sync_provider_reauth({provider, new_token})`
```

### Escenario 15: Multi-tenant scope estricto

```gherkin
Dado que existen 2 orgs con distintos provider_configs Jira (Jira Cloud distintos)
Cuando push corre
Entonces driver usa el provider_config de la org del entity (REQ/HU)
Y nunca cross-orga (RBAC enforce + tests dedicated)
```

## Esquema BD

```sql
CREATE TABLE provider_configs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  provider VARCHAR(30) NOT NULL,
    -- jira | github_issue | linear | asana | gitlab_issue
  display_name TEXT NOT NULL,
  config JSONB NOT NULL,
    -- jira: { url, project_key, default_issuetype_req, default_issuetype_hu, custom_field_*, epic_link_method, default_labels }
  credentials_ref TEXT NOT NULL,
    -- referencia a secret en REQ-02.3 secrets vault, NO el token raw
  active BOOLEAN NOT NULL DEFAULT true,
  last_health_check_at TIMESTAMPTZ,
  last_health_status VARCHAR(20),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (organization_id, provider, display_name)
);
CREATE INDEX ON provider_configs (organization_id, provider) WHERE active = true;

CREATE TABLE external_sync_state (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  entity_kind VARCHAR(10) NOT NULL,         -- req | hu
  entity_id UUID NOT NULL,                  -- FK soft a requirements.id o issues.id
  provider_config_id UUID NOT NULL REFERENCES provider_configs(id),
  external_key TEXT NOT NULL,               -- ej DIDE-145
  external_url TEXT NOT NULL,
  external_id TEXT,                         -- internal id si difiere de key
  sync_direction VARCHAR(20) NOT NULL DEFAULT 'push_only',
    -- push_only | pull_only | bidirectional
  sync_status VARCHAR(20) NOT NULL DEFAULT 'ok',
    -- ok | stale | conflict | error | auth_error | deleted_remote | partial
  field_mapping JSONB NOT NULL DEFAULT '{}',
    -- { last_pushed_values: { title, description, gherkin, ... }, custom_field_ids: {...} }
  drift_detected_at TIMESTAMPTZ,
  drift_fields JSONB,
  retries INT NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMPTZ,
  last_synced_at TIMESTAMPTZ,
  archived BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (provider_config_id, external_key)
);
CREATE INDEX ON external_sync_state (entity_kind, entity_id);
CREATE INDEX ON external_sync_state (organization_id, sync_status);
CREATE INDEX ON external_sync_state (next_retry_at) WHERE sync_status IN ('error', 'stale');

CREATE TABLE external_sync_events (
  id BIGSERIAL PRIMARY KEY,
  sync_state_id UUID REFERENCES external_sync_state(id) ON DELETE CASCADE,
  direction VARCHAR(10) NOT NULL,           -- push | pull
  operation VARCHAR(20) NOT NULL,           -- create | update | delete | status_change | comment
  request_summary JSONB,
  response_summary JSONB,
  ok BOOLEAN NOT NULL,
  error_message TEXT,
  duration_ms INT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON external_sync_events (sync_state_id, created_at DESC);
```

## MCP tools

| tool | input | output |
|------|-------|--------|
| `domain_sync_push` | `{entity: "req"\|"hu", entity_id, provider}` | `{external_key, external_url, sync_state_id}` |
| `domain_sync_get_state` | `{entity_kind, entity_id, provider?}` | `{sync_state[]}` |
| `domain_sync_resolve_conflict` | `{sync_state_id, resolution, manual_values?}` | `{ok, new_status}` |
| `domain_sync_retry` | `{sync_state_id, force?}` | `{queued: true}` |
| `domain_sync_provider_health` | `{provider_config_id?, provider?}` | `{ok, latency_ms, auth_valid, rate_limit_remaining}` |
| `domain_sync_provider_reauth` | `{provider_config_id, new_credentials_ref}` | `{ok}` |
| `domain_sync_drift_list` | `{since?, provider?}` | `{conflicts[]}` |

## Driver interface

```go
// internal/sdd/sync/driver/driver.go
type Driver interface {
    Name() string
    HealthCheck(ctx context.Context, cfg ProviderConfig) (HealthStatus, error)
    PushReq(ctx context.Context, cfg ProviderConfig, req Req) (RemoteRef, error)
    PushHU(ctx context.Context, cfg ProviderConfig, hu HU, parentRef *RemoteRef) (RemoteRef, error)
    UpdateRemote(ctx context.Context, cfg ProviderConfig, ref RemoteRef, diff FieldDiff) error
    UploadAttachments(ctx context.Context, cfg ProviderConfig, ref RemoteRef, attachments []Attachment) ([]RemoteAttachment, error)
    HandleWebhook(ctx context.Context, cfg ProviderConfig, payload []byte, headers map[string]string) ([]WebhookEvent, error)
    StatusMapping() map[string]string  // local→remote
}
```

## Webhook endpoint

`POST /webhooks/providers/:provider` con HMAC verification (REQ-10.2). Headers + body → driver `HandleWebhook` → eventos parseados → state machine apply.

## Análisis breve

- **Qué pide:** sync robusto Domain↔external (Jira MVP) con drift detection y resolución manual; queue async + retry/DLQ; multi-driver future-ready
- **Módulos sospechados:** `internal/sdd/sync/{service, queue, driver}/` + `internal/store/pg/sync/` + `internal/sdd/sync/driver/jira/`
- **Dependencias hard:** issue-04.1, issue-04.2, issue-04.6, REQ-02.3 (secrets para credentials), REQ-02.4 (audit), REQ-10.2 (webhook receiver con HMAC), REQ-26.5 (circuit breaker)
- **Riesgos:** drift no detectado por campos no monitoreados → solo monitoreamos title/description/AC; comments sync siempre pull (no push para evitar spam); rate limit Jira agresivo (429) → respetar Retry-After estrictamente; tokens expiran silenciosamente → healthcheck periódico
- **Esfuerzo tentativo:** XL
