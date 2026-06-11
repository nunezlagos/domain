# Tasks: issue-04.9-external-provider-sync

> **RE-SCOPE 2026-06-11 (decisión MCP-first 2026-06-10):** el core está
> implementado y la HU se cierra con este alcance — tablas external_providers
> + external_sync_state + external_sync_events (audit inmutable), Service
> (RegisterProvider/RegisterPush/MarkDrift/MarkResolved/MarkPartial/Get/
> GetByEntity/ListConflicts) y MCP tools domain_sync_* wireadas.
> El modelo: el AGENTE (que ya tiene acceso a Jira/GitHub vía sus propios
> MCP servers) hace el push/pull y registra el estado en Domain — Domain
> es el registro de verdad del sync, no el ejecutor.
> DIFERIDO: driver Jira HTTP + ADF renderer, driver GitHub Issues, webhooks
> pull con HMAC, workers async push/pull, status mapping bidireccional,
> HTTP endpoints. Checkboxes de esos bloques sin marcar a propósito.

## Schema

- [ ] **sy-001**: Migration `provider_configs`
- [ ] **sy-002**: Migration `external_sync_state`
- [ ] **sy-003**: Migration `external_sync_events`
- [ ] **sy-004**: Trigger `updated_at` en provider_configs + external_sync_state
- [ ] **sy-005**: Índice partial para `next_retry_at` solo en rows pendientes

## Store

- [ ] **sy-010**: Package `internal/store/pg/sync/`
- [ ] **sy-011**: CRUD `provider_configs`
- [ ] **sy-012**: CRUD `external_sync_state` (Create, Get, ListByEntity, ListPendingRetry)
- [ ] **sy-013**: `MarkDrift`, `ResolveConflict`, `MarkAuthError`, `MarkDeletedRemote`
- [ ] **sy-014**: `ClaimNextRetry` (CAS con next_retry_at < now)
- [ ] **sy-015**: Append `external_sync_events` audit

## Driver interface

- [ ] **sy-020**: Package `internal/sdd/sync/driver/`
- [ ] **sy-021**: Tipos compartidos: `ProviderConfig`, `Req`, `HU`, `RemoteRef`, `FieldDiff`, `Attachment`, `RemoteAttachment`, `HealthStatus`, `WebhookEvent`
- [ ] **sy-022**: Interface `Driver` definida
- [ ] **sy-023**: Registry de drivers (`driver.Register("jira", &JiraDriver{})`)
- [ ] **sy-024**: Resolución por nombre con error claro si missing

## Driver Jira

- [ ] **sy-030**: Package `internal/sdd/sync/driver/jira/`
- [ ] **sy-031**: HTTP client wrapper con auth header builder
- [ ] **sy-032**: `ratelimit.go` — tracker remaining + Retry-After parser
- [ ] **sy-033**: `adf.go` — convertidor markdown→ADF (heading, paragraph, codeBlock, list, link, image, mediaSingle)
- [ ] **sy-034**: `adf_test.go` — corpus de markdowns reales
- [ ] **sy-035**: `mapping.go` — read provider_config.config + field IDs
- [ ] **sy-036**: `transitions.go` — status mapping bidirectional
- [ ] **sy-037**: `PushReq` (Epic)
- [ ] **sy-038**: `PushHU` (Story con parent=epic_key)
- [ ] **sy-039**: `UpdateRemote` (PUT solo con campos cambiados)
- [ ] **sy-040**: `UploadAttachments` secuencial 1..N + binding media IDs
- [ ] **sy-041**: `attachments.go` — multipart builder + X-Atlassian-Token header
- [ ] **sy-042**: `HealthCheck` (GET /rest/api/3/myself)
- [ ] **sy-043**: `webhook.go` — parse jira:issue_updated changelog → []WebhookEvent
- [ ] **sy-044**: Detección de "echo" (cambio causado por Domain reciente <60s)

## Service

- [ ] **sy-050**: Package `internal/sdd/sync/` con Service skeleton
- [ ] **sy-051**: `Service.Push(entity, entity_id, provider, opts)` enqueue
- [ ] **sy-052**: `Service.GetState(entity_kind, entity_id, provider?)`
- [ ] **sy-053**: `Service.ResolveConflict(sync_state_id, resolution, manual_values?)`
- [ ] **sy-054**: `Service.Retry(sync_state_id, force?)`
- [ ] **sy-055**: `Service.ProviderHealth(provider_config_id?)`
- [ ] **sy-056**: `Service.ProviderReauth(provider_config_id, new_credentials_ref)`
- [ ] **sy-057**: `Service.DriftList(filter)`

## Worker queue

- [ ] **sy-060**: Worker pool config `DOMAIN_SYNC_WORKERS` default 2
- [ ] **sy-061**: LISTEN canal `sync_queue`
- [ ] **sy-062**: Tick recovery loop cada 30s (busca pending_retry)
- [ ] **sy-063**: Process op: ClaimNextRetry + driver call + state update + audit
- [ ] **sy-064**: Backoff calculator 30s/2m/10m/1h/4h
- [ ] **sy-065**: Move to DLQ tras 5 retries
- [ ] **sy-066**: Pause queue por provider si rate_limit_remaining < 10 o health=down

## Drift detection

- [ ] **sy-070**: `drift.go` — comparador campo a campo
- [ ] **sy-071**: Normalizadores (whitespace, HTML entities, ADF→plain)
- [ ] **sy-072**: Echo detection (cambio causado por Domain reciente)
- [ ] **sy-073**: Persist drift_fields como JSON con before/after

## Conflict resolution

- [ ] **sy-080**: `resolver.go` con 3 modos
- [ ] **sy-081**: accept_external → update issues + last_pushed_values
- [ ] **sy-082**: keep_local_force_push → driver.UpdateRemote
- [ ] **sy-083**: manual_merge → apply manual_values bilateralmente

## Webhook

- [ ] **sy-090**: Route `POST /webhooks/providers/:provider` registrado en HTTP server
- [ ] **sy-091**: HMAC verification delega a REQ-10.2
- [ ] **sy-092**: Provider router → driver.HandleWebhook
- [ ] **sy-093**: Replay protection (webhook_id cached 1h)
- [ ] **sy-094**: Event handler dispatcher (status_change | drift | comment | deleted_remote)

## MCP tools

- [ ] **sy-100**: `domain_sync_push`
- [ ] **sy-101**: `domain_sync_get_state`
- [ ] **sy-102**: `domain_sync_resolve_conflict`
- [ ] **sy-103**: `domain_sync_retry`
- [ ] **sy-104**: `domain_sync_provider_health`
- [ ] **sy-105**: `domain_sync_provider_reauth`
- [ ] **sy-106**: `domain_sync_drift_list`

## Eventos publicados

- [ ] **sy-110**: `sync.pushed` cuando push exitoso
- [ ] **sy-111**: `sync.drift_detected` cuando drift
- [ ] **sy-112**: `sync.resolved` cuando conflict resuelto
- [ ] **sy-113**: `sync.deleted_remote` cuando 404 desde provider
- [ ] **sy-114**: `sync.auth_error` cuando 401

## Notifications

- [ ] **sy-120**: Notif drift_detected al owner del HU + admin org (email + slack)
- [ ] **sy-121**: Notif auth_error al admin org (urgente)
- [ ] **sy-122**: Notif deleted_remote al owner

## Métricas / Tracing

- [ ] **sy-130**: `sync_push_total{provider, entity_kind, result}`
- [ ] **sy-131**: `sync_push_duration_seconds{provider, entity_kind}`
- [ ] **sy-132**: `sync_drift_detected_total{provider, field}`
- [ ] **sy-133**: `sync_queue_depth{provider}`
- [ ] **sy-134**: `sync_dlq_size{provider}`
- [ ] **sy-135**: `sync_provider_health{provider}` gauge
- [ ] **sy-136**: `sync_rate_limit_remaining{provider}` gauge
- [ ] **sy-137**: Tracing spans por op con atributos sync_state_id, provider
- [ ] **sy-138**: Logs estructurados con sync_state_id

## Tests

- [ ] **sy-200**: Unit ADF converter (markdown corpus)
- [ ] **sy-201**: Unit drift detector (no_drift, drift_simple, drift_multifield, echo_ignored)
- [ ] **sy-202**: Unit transitions mapping bidirectional
- [ ] **sy-203**: Unit ratelimit Retry-After parser
- [ ] **sy-204**: Integration push REQ Jira (WireMock)
- [ ] **sy-205**: Integration push HU + 3 attachments con media inline
- [ ] **sy-206**: Integration webhook status_change → issues.status updated
- [ ] **sy-207**: Integration webhook drift → conflict + notif
- [ ] **sy-208**: Integration resolve accept_external
- [ ] **sy-209**: Integration resolve keep_local_force_push
- [ ] **sy-210**: Integration retry exhausted → DLQ
- [ ] **sy-211**: Sabotaje 429 + Retry-After
- [ ] **sy-212**: Sabotaje 401 → auth_error, no retries
- [ ] **sy-213**: Sabotaje webhook replay → ignorado
- [ ] **sy-214**: Sabotaje cross-tenant (org A intenta usar provider org B) → 403
- [ ] **sy-215**: Concurrent push 100 ops paralelos → no race
- [ ] **sy-216**: Echo detection (push reciente + webhook → no drift)
- [ ] **sy-217**: 2 humanos edit Domain+Jira simultáneo → detecta conflict claro

## Documentación

- [ ] **sy-300**: `docs/sync/overview.md` con diagrama push+pull+drift
- [ ] **sy-301**: `docs/sync/provider-jira-setup.md` cómo configurar provider_config Jira
- [ ] **sy-302**: `docs/sync/conflict-resolution.md` con flowchart de las 3 opciones
- [ ] **sy-303**: Runbook `docs/runbooks/sync-stuck.md` qué hacer si queue se atasca
- [ ] **sy-304**: Runbook `docs/runbooks/sync-auth-expired.md` cómo rotar tokens
