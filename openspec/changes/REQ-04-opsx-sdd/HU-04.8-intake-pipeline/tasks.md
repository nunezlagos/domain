# Tasks: HU-04.8-intake-pipeline

## Schema

- [ ] **ip-001**: Migration `intake_payloads` con índices y constraint unique de idempotency
- [ ] **ip-002**: Migration `intake_to_req_links`
- [ ] **ip-003**: Trigger `updated_at` en intake_payloads
- [ ] **ip-004**: Función `intake_purge_expired_staging()` para cron (purga staged_attachments sin commit >7 días)

## Store

- [ ] **ip-010**: Package `internal/store/pg/intake/` skeleton
- [ ] **ip-011**: `CreateIntake`, `GetIntake`, `ListIntakes`, `UpdateStatus`
- [ ] **ip-012**: `UpdateClassification`, `UpdateDedupCandidates`, `UpdateProposedDraft`
- [ ] **ip-013**: `MarkHeartbeat`, `ClaimStaleIntake` (CAS contra last_heartbeat_at)
- [ ] **ip-014**: `CommitIntake` (transaction REQ+HU+attachments+links)
- [ ] **ip-015**: `RejectIntake`, `RetryIntake`

## Pipeline service

- [ ] **ip-020**: Package `internal/sdd/intake/` skeleton (Service, types, errors)
- [ ] **ip-021**: State machine enum + transition table
- [ ] **ip-022**: Service.Submit (valida payload + crea row received + sube staged_attachments)
- [ ] **ip-023**: Service.GetReview (assembler con preview Markdown)
- [ ] **ip-024**: Service.Approve (action=create_new) → CommitIntake transaction
- [ ] **ip-025**: Service.Approve (action=merge) → append a HU target
- [ ] **ip-026**: Service.Reject
- [ ] **ip-027**: Service.List, Service.Retry
- [ ] **ip-028**: Idempotency key handling (lookup before insert)
- [ ] **ip-029**: Payload validator (size, mime, URL no-SSRF)

## Steps

- [ ] **ip-030**: step `ingest` — persiste row, stage attachments a S3, marca `status=received`
- [ ] **ip-031**: step `classify` — invoca skill `intake.classify`, persiste `classified_*`
- [ ] **ip-032**: step `dedupe` — pgvector cosine sim contra `requirements.embedding`, persiste candidates
- [ ] **ip-033**: step `structure` — invoca skill `intake.structure`, persiste `proposed_hu_draft`
- [ ] **ip-034**: transición a `pending_review` + emit evento

## Worker

- [ ] **ip-040**: Worker pool config (DOMAIN_INTAKE_WORKERS=4 default)
- [ ] **ip-041**: LISTEN canal `intake_new`
- [ ] **ip-042**: Tick recovery loop (cada 30s busca status=processing con heartbeat stale)
- [ ] **ip-043**: Heartbeat updater (cada 5s mientras step en curso)
- [ ] **ip-044**: Graceful shutdown (espera step actual o checkpoint)

## Skills builtin

- [ ] **ip-050**: `intake.classify` skill + prompt template + few-shot examples
- [ ] **ip-051**: `intake.structure` skill + prompt template + few-shot (3 ejemplos por tipo)
- [ ] **ip-052**: Skill output schemas (JSON schema validation)
- [ ] **ip-053**: Skill version 1.0.0 publicada en registry

## MCP tools

- [ ] **ip-060**: `domain_intake_submit` tool
- [ ] **ip-061**: `domain_intake_get_review` tool (incluye preview Markdown render)
- [ ] **ip-062**: `domain_intake_approve` tool
- [ ] **ip-063**: `domain_intake_reject` tool
- [ ] **ip-064**: `domain_intake_list` tool con paginación
- [ ] **ip-065**: `domain_intake_retry` tool

## Eventos

- [ ] **ip-070**: Publisher para `intake.received`, `intake.ready_for_review`, `intake.committed`, `intake.rejected`
- [ ] **ip-071**: Schema versionado de cada event payload

## Notifications

- [ ] **ip-080**: Suscriber a `intake.ready_for_review` → email (REQ-20.2) con template
- [ ] **ip-081**: Suscriber a `intake.ready_for_review` → slack (REQ-20.3) con interactive buttons

## Adapters interface

- [ ] **ip-090**: Declarar interfaz `IntakeSubmitter` en `internal/sdd/intake/adapters.go`
- [ ] **ip-091**: Documentar contrato esperado de cada adapter futuro
- [ ] **ip-092**: Helper `NormalizePayload(source, raw) (SubmitInput, error)` reusable

## Multi-tenant + RBAC

- [ ] **ip-100**: Permission `intake:submit` definida
- [ ] **ip-101**: Permission `intake:approve` definida
- [ ] **ip-102**: Permission `intake:read` definida
- [ ] **ip-103**: Enforce org_id en todas las queries

## Quotas

- [ ] **ip-110**: Soft cap 800 intakes/día/org → notif admin
- [ ] **ip-111**: Hard cap 1000 intakes/día/org → 429 con Retry-After
- [ ] **ip-112**: Métrica `intake_quota_usage_ratio{organization_id}`

## Métricas / Tracing / Logging

- [ ] **ip-120**: Counter `intake_submitted_total{source, type, severity}`
- [ ] **ip-121**: Counter `intake_processed_total{source, final_status}`
- [ ] **ip-122**: Histogram `intake_step_duration_seconds{step}`
- [ ] **ip-123**: Counter `intake_dedup_hit_total{action}`
- [ ] **ip-124**: Histogram `intake_classification_confidence_bucket{source}`
- [ ] **ip-125**: Tracing span por step con atributos intake_id, organization_id
- [ ] **ip-126**: Structured logs con campos intake_id, source, step, status

## Tests

- [ ] **ip-200**: Unit tests por step (mocks LLM/store/S3)
- [ ] **ip-201**: Unit tests state machine transitions válidas/inválidas
- [ ] **ip-202**: Integration test end-to-end con testcontainers (PG + MinIO + LLM fake)
- [ ] **ip-203**: Integration test crash recovery (kill worker mid-pipeline)
- [ ] **ip-204**: Integration test concurrent take-over (2 workers, 1 intake)
- [ ] **ip-205**: Integration test idempotency_key duplicate
- [ ] **ip-206**: Integration test approve con action=create_new + edits
- [ ] **ip-207**: Integration test approve con action=merge a HU existente
- [ ] **ip-208**: Sabotaje tests (payload size, mime invalido, SSRF URL)
- [ ] **ip-209**: Load test 100 submits paralelos → no race conditions

## Documentación

- [ ] **ip-300**: `docs/intake/overview.md` con diagrama de pipeline
- [ ] **ip-301**: `docs/intake/skill-prompts.md` con prompts versionados
- [ ] **ip-302**: `docs/intake/adapters-spec.md` contrato para adapters futuros
- [ ] **ip-303**: Runbook `docs/runbooks/intake-stuck.md` qué hacer si intakes quedan en processing

## Sabotaje verification

- [ ] **ip-400**: Mata worker durante step `structure` → verifica recovery
- [ ] **ip-401**: LLM responde basura → verifica que approve permite editar
- [ ] **ip-402**: Approve fails (FK violation) → verifica rollback completo
- [ ] **ip-403**: 2 humanos approve mismo intake simultáneo → 409 second loses
- [ ] **ip-404**: S3 caído al stage attachment → reintento con backoff, error claro al user
