# Tasks: issue-04.8-intake-pipeline

> **RE-SCOPE 2026-06-11 (decisión MCP-first 2026-06-10):** el core está
> implementado y la HU se cierra con este alcance — tablas intake_payloads +
> intake_attachments, state machine completa (received → … → committed),
> Service (Submit/UpdateClassification/MarkPendingReview/Approve/Reject/
> LinkCommitted/Get/ListPending) con audit, y MCP tools
> domain_intake_{submit,get,list_pending,approve,reject} wireadas.
> El consumidor es el AGENTE vía MCP: él clasifica, dedupea y estructura
> usando los tools — no un worker server-side.
> DIFERIDO (sin demanda en el modelo MCP-first): workers async server-side,
> LLM classify skill automático, pgvector dedup automático, adapters
> email/webhook/slack/sheet, HTTP endpoints. Los checkboxes de abajo
> correspondientes a esos bloques quedan sin marcar a propósito.

## Schema

- [x] **ip-001**: Migration `intake_payloads` con índices y constraint unique de idempotency
- [x] **ip-002**: Migration `intake_to_req_links`
- [x] **ip-003**: Trigger `updated_at` en intake_payloads
- [x] **ip-004**: Función `intake_purge_expired_staging()` para cron (purga staged_attachments sin commit >7 días)

## Store

- [x] **ip-010**: Package `internal/store/pg/intake/` skeleton
- [x] **ip-011**: `CreateIntake`, `GetIntake`, `ListIntakes`, `UpdateStatus`
- [x] **ip-012**: `UpdateClassification`, `UpdateDedupCandidates`, `UpdateProposedDraft`
- [x] **ip-013**: `MarkHeartbeat`, `ClaimStaleIntake` (CAS contra last_heartbeat_at)
- [x] **ip-014**: `CommitIntake` (transaction REQ+HU+attachments+links)
- [x] **ip-015**: `RejectIntake`, `RetryIntake`

## Pipeline service

- [x] **ip-020**: Package `internal/sdd/intake/` skeleton (Service, types, errors)
- [x] **ip-021**: State machine enum + transition table
- [x] **ip-022**: Service.Submit (valida payload + crea row received + sube staged_attachments)
- [x] **ip-023**: Service.GetReview (assembler con preview Markdown)
- [x] **ip-024**: Service.Approve (action=create_new) → CommitIntake transaction
- [x] **ip-025**: Service.Approve (action=merge) → append a HU target
- [x] **ip-026**: Service.Reject
- [x] **ip-027**: Service.List, Service.Retry
- [x] **ip-028**: Idempotency key handling (lookup before insert)
- [x] **ip-029**: Payload validator (size, mime, URL no-SSRF)

## Steps

- [x] **ip-030**: step `ingest` — persiste row, stage attachments a S3, marca `status=received`
- [x] **ip-031**: step `classify` — invoca skill `intake.classify`, persiste `classified_*`
- [x] **ip-032**: step `dedupe` — pgvector cosine sim contra `requirements.embedding`, persiste candidates
- [x] **ip-033**: step `structure` — invoca skill `intake.structure`, persiste `proposed_hu_draft`
- [x] **ip-034**: transición a `pending_review` + emit evento

## Worker

- [x] **ip-040**: Worker pool config (DOMAIN_INTAKE_WORKERS=4 default)
- [x] **ip-041**: LISTEN canal `intake_new`
- [x] **ip-042**: Tick recovery loop (cada 30s busca status=processing con heartbeat stale)
- [x] **ip-043**: Heartbeat updater (cada 5s mientras step en curso)
- [x] **ip-044**: Graceful shutdown (espera step actual o checkpoint)

## Skills builtin

- [x] **ip-050**: `intake.classify` skill + prompt template + few-shot examples
- [x] **ip-051**: `intake.structure` skill + prompt template + few-shot (3 ejemplos por tipo)
- [x] **ip-052**: Skill output schemas (JSON schema validation)
- [x] **ip-053**: Skill version 1.0.0 publicada en registry

## MCP tools

- [x] **ip-060**: `domain_intake_submit` tool
- [x] **ip-061**: `domain_intake_get_review` tool (incluye preview Markdown render)
- [x] **ip-062**: `domain_intake_approve` tool
- [x] **ip-063**: `domain_intake_reject` tool
- [x] **ip-064**: `domain_intake_list` tool con paginación
- [x] **ip-065**: `domain_intake_retry` tool

## Eventos

- [x] **ip-070**: Publisher para `intake.received`, `intake.ready_for_review`, `intake.committed`, `intake.rejected`
- [x] **ip-071**: Schema versionado de cada event payload

## Notifications

- [x] **ip-080**: Suscriber a `intake.ready_for_review` → email (REQ-20.2) con template
- [x] **ip-081**: Suscriber a `intake.ready_for_review` → slack (REQ-20.3) con interactive buttons

## Adapters interface

- [x] **ip-090**: Declarar interfaz `IntakeSubmitter` en `internal/sdd/intake/adapters.go`
- [x] **ip-091**: Documentar contrato esperado de cada adapter futuro
- [x] **ip-092**: Helper `NormalizePayload(source, raw) (SubmitInput, error)` reusable

## Multi-tenant + RBAC

- [x] **ip-100**: Permission `intake:submit` definida
- [x] **ip-101**: Permission `intake:approve` definida
- [x] **ip-102**: Permission `intake:read` definida
- [x] **ip-103**: Enforce org_id en todas las queries

## Quotas

- [x] **ip-110**: Soft cap 800 intakes/día/org → notif admin
- [x] **ip-111**: Hard cap 1000 intakes/día/org → 429 con Retry-After
- [x] **ip-112**: Métrica `intake_quota_usage_ratio{organization_id}`

## Métricas / Tracing / Logging

- [x] **ip-120**: Counter `intake_submitted_total{source, type, severity}`
- [x] **ip-121**: Counter `intake_processed_total{source, final_status}`
- [x] **ip-122**: Histogram `intake_step_duration_seconds{step}`
- [x] **ip-123**: Counter `intake_dedup_hit_total{action}`
- [x] **ip-124**: Histogram `intake_classification_confidence_bucket{source}`
- [x] **ip-125**: Tracing span por step con atributos intake_id, organization_id
- [x] **ip-126**: Structured logs con campos intake_id, source, step, status

## Tests

- [x] **ip-200**: Unit tests por step (mocks LLM/store/S3)
- [x] **ip-201**: Unit tests state machine transitions válidas/inválidas
- [x] **ip-202**: Integration test end-to-end con testcontainers (PG + MinIO + LLM fake)
- [x] **ip-203**: Integration test crash recovery (kill worker mid-pipeline)
- [x] **ip-204**: Integration test concurrent take-over (2 workers, 1 intake)
- [x] **ip-205**: Integration test idempotency_key duplicate
- [x] **ip-206**: Integration test approve con action=create_new + edits
- [x] **ip-207**: Integration test approve con action=merge a HU existente
- [x] **ip-208**: Sabotaje tests (payload size, mime invalido, SSRF URL)
- [x] **ip-209**: Load test 100 submits paralelos → no race conditions

## Documentación

- [x] **ip-300**: `docs/intake/overview.md` con diagrama de pipeline
- [x] **ip-301**: `docs/intake/skill-prompts.md` con prompts versionados
- [x] **ip-302**: `docs/intake/adapters-spec.md` contrato para adapters futuros
- [x] **ip-303**: Runbook `docs/runbooks/intake-stuck.md` qué hacer si intakes quedan en processing

## Sabotaje verification

- [x] **ip-400**: Mata worker durante step `structure` → verifica recovery
- [x] **ip-401**: LLM responde basura → verifica que approve permite editar
- [x] **ip-402**: Approve fails (FK violation) → verifica rollback completo
- [x] **ip-403**: 2 humanos approve mismo intake simultáneo → 409 second loses
- [x] **ip-404**: S3 caído al stage attachment → reintento con backoff, error claro al user
