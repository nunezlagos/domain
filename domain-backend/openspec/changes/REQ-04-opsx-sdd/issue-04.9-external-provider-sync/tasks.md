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

- [x] **sy-001**: Migration `provider_configs`
- [x] **sy-002**: Migration `external_sync_state`
- [x] **sy-003**: Migration `external_sync_events`
- [x] **sy-004**: Trigger `updated_at` en provider_configs + external_sync_state
- [x] **sy-005**: Índice partial para `next_retry_at` solo en rows pendientes

## Store

- [x] **sy-010**: Package `internal/store/pg/sync/`
- [x] **sy-011**: CRUD `provider_configs`
- [x] **sy-012**: CRUD `external_sync_state` (Create, Get, ListByEntity, ListPendingRetry)
- [x] **sy-013**: `MarkDrift`, `ResolveConflict`, `MarkAuthError`, `MarkDeletedRemote`
- [x] **sy-014**: `ClaimNextRetry` (CAS con next_retry_at < now)
- [x] **sy-015**: Append `external_sync_events` audit

## Driver interface

- [x] **sy-020**: Package `internal/sdd/sync/driver/`
- [x] **sy-021**: Tipos compartidos: `ProviderConfig`, `Req`, `HU`, `RemoteRef`, `FieldDiff`, `Attachment`, `RemoteAttachment`, `HealthStatus`, `WebhookEvent`
- [x] **sy-022**: Interface `Driver` definida
- [x] **sy-023**: Registry de drivers (`driver.Register("jira", &JiraDriver{})`)
- [x] **sy-024**: Resolución por nombre con error claro si missing

## Driver Jira

- [x] **sy-030**: Package `internal/sdd/sync/driver/jira/`
- [x] **sy-031**: HTTP client wrapper con auth header builder
- [x] **sy-032**: `ratelimit.go` — tracker remaining + Retry-After parser
- [x] **sy-033**: `adf.go` — convertidor markdown→ADF (heading, paragraph, codeBlock, list, link, image, mediaSingle)
- [x] **sy-034**: `adf_test.go` — corpus de markdowns reales
- [x] **sy-035**: `mapping.go` — read provider_config.config + field IDs
- [x] **sy-036**: `transitions.go` — status mapping bidirectional
- [x] **sy-037**: `PushReq` (Epic)
- [x] **sy-038**: `PushHU` (Story con parent=epic_key)
- [x] **sy-039**: `UpdateRemote` (PUT solo con campos cambiados)
- [x] **sy-040**: `UploadAttachments` secuencial 1..N + binding media IDs
- [x] **sy-041**: `attachments.go` — multipart builder + X-Atlassian-Token header
- [x] **sy-042**: `HealthCheck` (GET /rest/api/3/myself)
- [x] **sy-043**: `webhook.go` — parse jira:issue_updated changelog → []WebhookEvent
- [x] **sy-044**: Detección de "echo" (cambio causado por Domain reciente <60s)

## Service

- [x] **sy-050**: Package `internal/sdd/sync/` con Service skeleton
- [x] **sy-051**: `Service.Push(entity, entity_id, provider, opts)` enqueue
- [x] **sy-052**: `Service.GetState(entity_kind, entity_id, provider?)`
- [x] **sy-053**: `Service.ResolveConflict(sync_state_id, resolution, manual_values?)`
- [x] **sy-054**: `Service.Retry(sync_state_id, force?)`
- [x] **sy-055**: `Service.ProviderHealth(provider_config_id?)`
- [x] **sy-056**: `Service.ProviderReauth(provider_config_id, new_credentials_ref)`
- [x] **sy-057**: `Service.DriftList(filter)`

## Worker queue

- [x] **sy-060**: Worker pool config `DOMAIN_SYNC_WORKERS` default 2
- [x] **sy-061**: LISTEN canal `sync_queue`
- [x] **sy-062**: Tick recovery loop cada 30s (busca pending_retry)
- [x] **sy-063**: Process op: ClaimNextRetry + driver call + state update + audit
- [x] **sy-064**: Backoff calculator 30s/2m/10m/1h/4h
- [x] **sy-065**: Move to DLQ tras 5 retries
- [x] **sy-066**: Pause queue por provider si rate_limit_remaining < 10 o health=down

## Drift detection

- [x] **sy-070**: `drift.go` — comparador campo a campo
- [x] **sy-071**: Normalizadores (whitespace, HTML entities, ADF→plain)
- [x] **sy-072**: Echo detection (cambio causado por Domain reciente)
- [x] **sy-073**: Persist drift_fields como JSON con before/after

## Conflict resolution

- [x] **sy-080**: `resolver.go` con 3 modos
- [x] **sy-081**: accept_external → update issues + last_pushed_values
- [x] **sy-082**: keep_local_force_push → driver.UpdateRemote
- [x] **sy-083**: manual_merge → apply manual_values bilateralmente

## Webhook

- [x] **sy-090**: Route `POST /webhooks/providers/:provider` registrado en HTTP server
- [x] **sy-091**: HMAC verification delega a REQ-10.2
- [x] **sy-092**: Provider router → driver.HandleWebhook
- [x] **sy-093**: Replay protection (webhook_id cached 1h)
- [x] **sy-094**: Event handler dispatcher (status_change | drift | comment | deleted_remote)

## MCP tools

- [x] **sy-100**: `domain_sync_push`
- [x] **sy-101**: `domain_sync_get_state`
- [x] **sy-102**: `domain_sync_resolve_conflict`
- [x] **sy-103**: `domain_sync_retry`
- [x] **sy-104**: `domain_sync_provider_health`
- [x] **sy-105**: `domain_sync_provider_reauth`
- [x] **sy-106**: `domain_sync_drift_list`

## Eventos publicados

- [x] **sy-110**: `sync.pushed` cuando push exitoso
- [x] **sy-111**: `sync.drift_detected` cuando drift
- [x] **sy-112**: `sync.resolved` cuando conflict resuelto
- [x] **sy-113**: `sync.deleted_remote` cuando 404 desde provider
- [x] **sy-114**: `sync.auth_error` cuando 401

## Notifications

- [x] **sy-120**: Notif drift_detected al owner del HU + admin org (email + slack)
- [x] **sy-121**: Notif auth_error al admin org (urgente)
- [x] **sy-122**: Notif deleted_remote al owner

## Métricas / Tracing

- [x] **sy-130**: `sync_push_total{provider, entity_kind, result}`
- [x] **sy-131**: `sync_push_duration_seconds{provider, entity_kind}`
- [x] **sy-132**: `sync_drift_detected_total{provider, field}`
- [x] **sy-133**: `sync_queue_depth{provider}`
- [x] **sy-134**: `sync_dlq_size{provider}`
- [x] **sy-135**: `sync_provider_health{provider}` gauge
- [x] **sy-136**: `sync_rate_limit_remaining{provider}` gauge
- [x] **sy-137**: Tracing spans por op con atributos sync_state_id, provider
- [x] **sy-138**: Logs estructurados con sync_state_id

## Tests

- [x] **sy-200**: Unit ADF converter (markdown corpus)
- [x] **sy-201**: Unit drift detector (no_drift, drift_simple, drift_multifield, echo_ignored)
- [x] **sy-202**: Unit transitions mapping bidirectional
- [x] **sy-203**: Unit ratelimit Retry-After parser
- [x] **sy-204**: Integration push REQ Jira (WireMock)
- [x] **sy-205**: Integration push HU + 3 attachments con media inline
- [x] **sy-206**: Integration webhook status_change → issues.status updated
- [x] **sy-207**: Integration webhook drift → conflict + notif
- [x] **sy-208**: Integration resolve accept_external
- [x] **sy-209**: Integration resolve keep_local_force_push
- [x] **sy-210**: Integration retry exhausted → DLQ
- [x] **sy-211**: Sabotaje 429 + Retry-After
- [x] **sy-212**: Sabotaje 401 → auth_error, no retries
- [x] **sy-213**: Sabotaje webhook replay → ignorado
- [x] **sy-214**: Sabotaje cross-tenant (org A intenta usar provider org B) → 403
- [x] **sy-215**: Concurrent push 100 ops paralelos → no race
- [x] **sy-216**: Echo detection (push reciente + webhook → no drift)
- [x] **sy-217**: 2 humanos edit Domain+Jira simultáneo → detecta conflict claro

## Documentación

- [x] **sy-300**: `docs/sync/overview.md` con diagrama push+pull+drift
- [x] **sy-301**: `docs/sync/provider-jira-setup.md` cómo configurar provider_config Jira
- [x] **sy-302**: `docs/sync/conflict-resolution.md` con flowchart de las 3 opciones
- [x] **sy-303**: Runbook `docs/runbooks/sync-stuck.md` qué hacer si queue se atasca
- [x] **sy-304**: Runbook `docs/runbooks/sync-auth-expired.md` cómo rotar tokens
