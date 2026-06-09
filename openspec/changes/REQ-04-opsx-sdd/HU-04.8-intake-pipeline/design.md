# Design: HU-04.8-intake-pipeline

## Decisión arquitectónica

**State machine persistida**, no memoria. Cada step escribe su output en columna dedicada de `intake_payloads`. Resumability built-in (resume desde último step completado tras crash). Sin event sourcing — basta con un row por intake + un audit log separado.

**Worker model**: 1 service Go con worker pool (default 4). Coordinación vía Postgres `LISTEN/NOTIFY` (canal `intake_new`) + tick cada 30s como fallback (recovery). No requiere Redis ni broker externo en MVP.

**Skills builtin para LLM steps** (`intake.classify`, `intake.structure`). Esto los hace versionables (REQ-05.3), reemplazables y testeables sin tocar el pipeline. El pipeline solo orquesta; el "qué pregunta hace al LLM" es config.

**Adapters out-of-scope** explícitamente. El primer caller es el agente IA (MCP). Los demás adapters (email/webhook/sheets/slack) tienen su HU dedicada cada uno con la interfaz `IntakeSubmitter` declarada en este servicio.

## Componentes

```
internal/sdd/intake/
  service.go           # Submit, GetReview, Approve, Reject, List, Retry
  pipeline.go          # state machine orchestration, step dispatcher
  steps_ingest.go      # step 1: persist row + stage attachments a S3
  steps_classify.go    # step 2: llm_call → type, severity, confidence
  steps_dedupe.go      # step 3: pgvector semantic search vs req embeddings
  steps_structure.go   # step 4: llm_call → title, description, gherkin
  steps_commit.go      # step 6: transaction REQ+HU+attachments
  worker.go            # worker pool con LISTEN/NOTIFY + tick recovery
  types.go             # Intake, Status, Step, Action enums
  events.go            # IntakeCreated, IntakeApproved, IntakeRejected publishers
  validators.go        # payload size, mime, action contracts
  adapters.go          # interfaz IntakeSubmitter para futuros adapters

internal/store/pg/intake/
  store.go             # CRUD pgx wrap
  queries.sql          # named queries para steps

internal/skills/builtin/intake/
  classify.go          # skill registration + prompt template
  structure.go         # skill registration + few-shot templates
  templates/           # go:embed *.tmpl

internal/mcp/tools/intake/
  submit.go
  get_review.go
  approve.go
  reject.go
  list.go
  retry.go
```

## State transitions

```
                   ┌─────────────┐
                   │  received   │  ← persist row, stage attachments
                   └──────┬──────┘
                          ▼
                   ┌─────────────┐
                   │ processing  │  ← worker toma + heartbeat
                   └──┬───┬───┬──┘
                      │   │   │
            classify──┘   │   └──structure
                          │
                       dedupe
                          │
                      ▼ (all done)
                   ┌─────────────┐
                   │pending_     │  ← notifica humano
                   │  review     │
                   └──┬───┬───┬──┘
              approve │   │   │ reject
                      │   │   └────────────┐
                      ▼   ▼                ▼
              ┌──────────────┐      ┌──────────┐
              │  committed   │      │ rejected │
              └──────────────┘      └──────────┘

   en cualquier momento error transitorio → "error" → retry → "processing"
```

## Transaction boundaries

**Commit (approve)** debe ser una sola transaction:
```sql
BEGIN;
  -- si action=create_new
  INSERT INTO requirements (...) RETURNING id;
  INSERT INTO user_stories (req_id=..., ...) RETURNING id;
  UPDATE attachments SET hu_id=... WHERE staged_intake_id=... ;
  -- si action=merge
  UPDATE user_stories SET description_md=..., gherkin_text=... WHERE id=:target;
  UPDATE attachments SET hu_id=:target WHERE staged_intake_id=... ;
  -- siempre
  INSERT INTO intake_to_req_links (...);
  UPDATE intake_payloads SET status='committed', resulting_req_id=..., resulting_hu_id=...;
  INSERT INTO audit_log (...);  -- vía REQ-02.4
  NOTIFY req_hu_created, ?json_payload;
COMMIT;
```

Si falla cualquier insert → rollback completo → intake vuelve a `pending_review` con `error_message`.

## Resumability

Cada step se ejecuta solo si `intake_payloads.{step_output_column} IS NULL`. Si crash entre step 2 y 3 → al retomar, worker hace `SELECT WHERE status='processing' AND classified_type IS NOT NULL AND dedup_candidates = '[]'` → corre solo dedupe en adelante.

`last_heartbeat_at` se updatea cada 5s mientras el worker procesa. Otro worker que vea `last_heartbeat_at < now() - 5min AND status='processing'` puede tomar el intake.

## Async vs sync responses

- `domain_intake_submit` es **async**: devuelve `intake_id` + `status: "processing"`. El humano luego pollea `domain_intake_get_review` o se suscribe al evento `intake.ready_for_review`.
- `domain_intake_approve` es **sync**: bloquea hasta que la transaction de commit termina. Si tarda > 30s → 503 retry-later.
- `domain_intake_reject` es **sync**: simple update.

## Notificación de "ready for review"

Cuando step "structure" termina y status pasa a `pending_review`:
1. Inserta evento `intake.ready_for_review` en event bus (REQ-10.3).
2. Suscriptores configurables vía REQ-20 (notifications):
   - email a `intake_review_emails` de la org
   - mensaje Slack a `intake_review_channel`
   - SSE para Web UI conectada
3. El payload del evento incluye `intake_id` + preview corto → humano puede actuar desde la notificación.

## Idempotencia

- `idempotency_key` único por `(organization_id, key)`.
- Para email adapter (futuro): `hash(message_id + from + subject)`.
- Para webhook genérico: `hash(payload + timestamp_5min_bucket)`.
- Para agent: el agente puede pasar `idempotency_key` explícito si quiere garantizar no-dupe en reintentos.

## Quotas / abuse

- Default 1.000 intakes/día/org.
- Soft cap → notificación al admin de la org cuando > 80%.
- Hard cap → 429 con `Retry-After`.

## Security

- Payload size limits ya documentados (256KB raw, 10MB attachment).
- Mime allowlist enforce en step ingest.
- Sanitización de raw_text antes de loggear (no PII en log estructurado, solo hash).
- API key con scope `intake:submit` distinto a `intake:approve` (RBAC REQ-02.2).
- S3 staging bucket lifecycle: purge >7 días si intake no committed.

## Observability

- Métricas (REQ-17.1):
  - `intake_submitted_total{source, type, severity}`
  - `intake_processed_total{source, final_status}`
  - `intake_step_duration_seconds{step}`
  - `intake_classification_confidence_bucket{source}`
  - `intake_dedup_hit_total{action}`
- Tracing (REQ-17.2): un trace por intake, span per step.
- Logging (REQ-17.3): structured con `intake_id`, `organization_id`, `source`, `step`.

## Testing strategy

- Unit: cada step aislado con mocks de LLM / S3 / store.
- Integration: testcontainers Postgres + minio + LLM fake → flujo completo.
- E2E sabotaje: payload max + payload malicioso + LLM timeout + crash recovery.

## Trade-offs explícitos

| Decisión | Trade-off |
|---|---|
| LISTEN/NOTIFY vs queue externa | Más simple en MVP, pero un solo Postgres = bottleneck si escala >10k intakes/día. Mover a NATS o Redis si necesario. |
| Single DB transaction en commit | Atomic pero limita el tamaño máx del HU (no podés crear 1000 attachments en 1 commit). En la práctica, intakes traen 1-10 attachments. |
| Adapters out-of-scope | Más HUs después pero mantiene esta tractable y testeable. |
| Dedup solo en step 3 (no en submit) | Submit es rápido (200ms). Dedup costoso (embeddings) sucede en worker async. Trade: idempotency_key cubre dupes obvios; dedup semántico cubre dupes "iguales con otras palabras". |
