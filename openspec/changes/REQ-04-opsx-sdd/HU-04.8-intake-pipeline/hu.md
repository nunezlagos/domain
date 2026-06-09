# HU-04.8-intake-pipeline

**Origen:** `REQ-04-opsx-sdd`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** PM/desarrollador que recibe requerimientos del cliente por canales heterogéneos (chat con agente IA, email, planilla, Slack, webhook)
**Quiero** una pipeline única que reciba el payload crudo, lo clasifique con IA, detecte duplicados, genere REQ+HU+Gherkin en BD, y me pida revisión antes de impactar Jira
**Para** que ningún requerimiento se pierda, todos queden auditados, el cliente reciba acuse de recibo automático, y la calidad del SDD no dependa de cómo llegó la petición

## MVP

La primera versión soporta **un único origen**: el agente IA (Claude Code) pega texto libre al MCP tool `domain_intake_submit`. Los demás adapters (gmail, webhook HTTP, sheets, slack) tienen su modelo en BD y endpoint reservado pero el código se entrega en HUs siguientes (out-of-scope esta HU; ver tasks "deferred").

## Diferencia con HU-04.7

| HU-04.7 wizard | HU-04.8 intake |
|---|---|
| Hace preguntas dirigidas al usuario | Recibe texto libre / payload sin estructura |
| User inicia con `domain_hu_create_start` | Sistema/agente inicia con `domain_intake_submit` |
| Cada paso requiere respuesta humana | IA hace 1ra pasada sola → genera draft → review humano |
| Output: HU bien formada lista para commit | Output: REQ+HU+Gherkin + estado `pending_review` esperando aprobación |
| Multi-step, conversacional | Single-shot inicial + 1 ciclo de review |

Las 2 HUs son **complementarias**: 04.7 sirve cuando ya sabés qué querés especificar; 04.8 sirve cuando llega ruido del exterior y querés estructurarlo.

## Pipeline conceptual

```
        ┌──────────────┐
        │ raw payload  │  (texto + adjuntos)
        └──────┬───────┘
               │ domain_intake_submit
               ▼
        ┌──────────────┐
        │ 1. ingest    │  → row en intake_payloads (status=received)
        └──────┬───────┘
               ▼
        ┌──────────────┐
        │ 2. classify  │  llm_call → type, severity, confidence
        └──────┬───────┘
               ▼
        ┌──────────────┐
        │ 3. dedupe    │  semantic search vía memory + req embeddings
        └──────┬───────┘
               ▼
        ┌──────────────┐
        │ 4. structure │  llm_call → title, description, gherkin AC
        └──────┬───────┘
               ▼
        ┌──────────────┐
        │ 5. review    │  status=pending_review → notifica humano
        └──────┬───────┘     espera approve/edit/reject/merge
               ▼
        ┌──────────────┐
        │ 6. commit    │  crea requirements + user_stories + attachments
        └──────┬───────┘     status=committed
               ▼
        ┌──────────────┐
        │ 7. external  │  dispara HU-04.9 (sync a Jira u otro provider)
        └──────────────┘
```

## Criterios de aceptación

### Escenario 1: Submit single-shot desde agente IA

```gherkin
Dado que un agente IA conectado al MCP de Domain
Cuando invoca `domain_intake_submit({source: "agent", raw_text: "El director no puede descargar la ficha aunque haya completado las 4 tasas, ya pasé el screenshot adjunto", attachments: [{name: "screenshot.png", b64: "..."}]})`
Entonces se crea row en `intake_payloads` con status="received" y se devuelve `{intake_id, status: "processing"}`
Y la pipeline procesa los 4 pasos (classify, dedupe, structure, review-ready) async
Y al finalizar la row queda en status="pending_review" con `classified_type`, `severity`, `proposed_req_id_or_new`, `proposed_hu_draft`, `dedup_candidates[]`
```

### Escenario 2: Clasificación automática

```gherkin
Dado que existe intake en status="received"
Cuando el worker procesa step "classify"
Entonces se invoca skill builtin `intake.classify` (llm_call con prompt cerrado)
Y devuelve `{type: "fix"|"feat"|"hotfix"|"chore"|"refactor"|"docs", severity: "low"|"medium"|"high"|"critical", confidence: 0..1, reasoning: text}`
Y se actualiza `intake_payloads.classified_*`
Y si confidence < 0.6 → marca `needs_clarification=true`
```

### Escenario 3: Dedupe contra REQs existentes

```gherkin
Dado que intake clasificado tiene `proposed_title` generado
Cuando worker procesa step "dedupe"
Entonces se hace búsqueda semántica (pgvector cosine) contra embeddings de `requirements.title + description`
Y se devuelven top-5 candidates con `similarity > 0.75`
Y se persisten en `intake_payloads.dedup_candidates` como `[{req_id, hu_id?, similarity, reason}]`
Y si similarity > 0.92 → sugiere `merge_action="append_to_hu"` en lugar de "create_new"
```

### Escenario 4: Generar HU + Gherkin draft

```gherkin
Dado que intake clasificado y deduplicado
Cuando worker procesa step "structure"
Entonces se invoca skill builtin `intake.structure` (llm_call con few-shot)
Y devuelve `{title, description_md, gherkin_scenarios[], suggested_priority, suggested_effort, suggested_audience, suggested_components[]}`
Y se persiste en `intake_payloads.proposed_hu_draft` como JSON
Y el draft NO crea row en `user_stories` todavía (solo el draft en intake_payloads)
```

### Escenario 5: Review pendiente — preview render

```gherkin
Dado que intake en status="pending_review"
Cuando humano invoca `domain_intake_get_review({intake_id})`
Entonces devuelve `{source, raw_payload, classified_type, severity, confidence, dedup_candidates, proposed_hu_draft, attachments_uploaded_keys, suggested_actions: ["approve","edit","reject","merge"]}`
Y la respuesta incluye markdown preview de cómo se vería la HU si se aprueba
```

### Escenario 6: Aprobar sin edits → commit

```gherkin
Dado que intake en pending_review
Cuando humano invoca `domain_intake_approve({intake_id, action: "create_new"})`
Entonces dentro de UNA transaction se crea:
  - row en requirements si action requiere nuevo REQ (o link a REQ existente)
  - row en user_stories con title, description, gherkin del draft
  - rows en attachments con los S3 keys ya subidos en step 1
  - row en intake_to_req_links con link_type="created"
Y se actualiza intake_payloads.status="committed", resulting_req_id, resulting_hu_id
Y se dispara evento "req.hu.created" al event bus (REQ-10.3)
Y NO se llama todavía a Jira (eso es HU-04.9 disparado por el evento)
```

### Escenario 7: Aprobar con edits

```gherkin
Dado que intake en pending_review
Cuando humano invoca `domain_intake_approve({intake_id, action: "create_new", edits: {title: "...", description_md: "...", gherkin: "..."}})`
Entonces el `proposed_hu_draft` se reemplaza por los edits
Y el commit usa los edits, no el draft original
Y se persiste `intake_payloads.human_edits=jsonb_diff(original, edits)` para audit
```

### Escenario 8: Mergear con HU existente (dupe detected)

```gherkin
Dado que dedup_candidates tiene match con similarity > 0.85
Cuando humano invoca `domain_intake_approve({intake_id, action: "merge", target_hu_id: "..."})`
Entonces NO se crea HU nueva
Y se appenden scenarios Gherkin del draft a la HU target
Y se appenden attachments a la HU target
Y se crea intake_to_req_links con link_type="merged"
Y se notifica al author original del HU target ("se agregaron N criterios desde intake X")
```

### Escenario 9: Rechazar (spam/duplicado exacto/no aplica)

```gherkin
Dado que intake en pending_review
Cuando humano invoca `domain_intake_reject({intake_id, reason: "spam"|"exact_duplicate"|"out_of_scope"|"other", note: text?})`
Entonces intake.status="rejected"
Y se persiste reason + note + rejected_by + rejected_at
Y NO se crea REQ/HU/attachments
Y si la fuente lo permite (email), se responde con plantilla amable
```

### Escenario 10: Adjuntos pre-subidos a S3 en ingest

```gherkin
Dado que el payload trae `attachments: [{name, b64|url}]`
Cuando se procesa step "ingest"
Entonces cada attachment se sube a S3 vía HU-04.6 al bucket "intake-staging/{intake_id}/"
Y se persiste `intake_payloads.staged_attachments=[{s3_key, sha256, size, mime}]`
Y al commit (escenario 6) los attachments se "mueven" lógicamente (rebind ownership) a attachments table
Y si commit rechazado → cron purga staging bucket >7 días
```

### Escenario 11: Pipeline async con resumability

```gherkin
Dado que pipeline se cae en step "structure" (timeout LLM)
Cuando worker se reinicia
Entonces detecta intakes en status="processing" con last_heartbeat > 5min
Y reanuda desde el step pendiente (no reprocesa los completados)
Y los outputs de steps previos (classify, dedupe) están persistidos como columnas separadas → idempotente
```

### Escenario 12: Multi-tenant + scope por org

```gherkin
Dado que existen organizations (REQ-21)
Cuando un intake llega
Entonces se asocia con la organization del API key / sesión
Y la búsqueda dedup solo mira REQs de la misma organization
Y la aprobación requiere permission `intake:approve` en esa org (REQ-02.2 RBAC)
```

### Escenario 13: Pre-existencia opcional de adapters

```gherkin
Dado que existen drivers adapter (email, webhook, sheets, slack) que aún NO están implementados (deferred a HUs siguientes)
Cuando estos adapters se construyan
Entonces todos llamarán internamente a `domain_intake_submit` con su payload normalizado
Y la pipeline NO cambia — solo el `source` y `metadata` varían
Y esta HU declara las interfaces pero no las implementaciones de adapters
```

### Escenario 14: Sabotaje — payload malicioso

```gherkin
Dado que payload trae raw_text con 50MB de basura, attachments con polyglot files o links HTTP a SSRF
Cuando se procesa ingest
Entonces tamaño máx raw_text=256KB → 413
Y attachment > 10MB → 413
Y mime types validados contra allowlist (image/* + pdf + text/*)
Y URLs en raw_text NO se siguen automáticamente (no SSRF)
```

### Escenario 15: Idempotencia

```gherkin
Dado que la fuente puede reenviar el mismo payload (email retry, webhook duplicate)
Cuando se invoca submit con `idempotency_key`
Entonces si ya existe intake con esa key + organization → devuelve el existente (200, no 201)
Y si la fuente lo permite, el adapter usa hash(message_id + from + subject) como key
```

## Esquema BD

```sql
CREATE TABLE intake_payloads (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  source VARCHAR(20) NOT NULL,
    -- agent|email|webhook|sheets|slack|cli
  source_subtype VARCHAR(40),
    -- gmail|outlook|github_webhook|generic_http|...
  idempotency_key TEXT,
  raw_payload JSONB NOT NULL,
    -- { text, subject?, from?, to?, message_id?, headers?, ... }
  staged_attachments JSONB NOT NULL DEFAULT '[]',
    -- [{ s3_key, filename, mime, size, sha256 }]
  status VARCHAR(20) NOT NULL DEFAULT 'received',
    -- received | processing | pending_review | approved | committed | rejected | error
  classified_type VARCHAR(20),
    -- feat|fix|hotfix|chore|refactor|docs|spike
  classified_severity VARCHAR(20),
    -- low|medium|high|critical
  classification_confidence NUMERIC(3,2),
  classification_reasoning TEXT,
  dedup_candidates JSONB NOT NULL DEFAULT '[]',
    -- [{ req_id, hu_id?, similarity, reason }]
  proposed_hu_draft JSONB,
    -- { title, description_md, gherkin, audience, priority, effort, components[] }
  human_edits JSONB,
    -- jsonb_diff del draft original vs los edits aplicados
  human_review_action VARCHAR(20),
    -- create_new|merge|reject
  human_review_target_hu_id UUID,
  human_review_by UUID REFERENCES users(id),
  human_review_at TIMESTAMPTZ,
  reject_reason VARCHAR(40),
  reject_note TEXT,
  resulting_req_id UUID REFERENCES requirements(id),
  resulting_hu_id UUID REFERENCES user_stories(id),
  last_heartbeat_at TIMESTAMPTZ,
  error_message TEXT,
  retries INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON intake_payloads (organization_id, status);
CREATE INDEX ON intake_payloads (organization_id, source, created_at DESC);
CREATE UNIQUE INDEX ON intake_payloads (organization_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX ON intake_payloads (status, last_heartbeat_at) WHERE status = 'processing';

CREATE TABLE intake_to_req_links (
  id BIGSERIAL PRIMARY KEY,
  intake_id UUID NOT NULL REFERENCES intake_payloads(id) ON DELETE CASCADE,
  req_id UUID REFERENCES requirements(id),
  hu_id UUID REFERENCES user_stories(id),
  link_type VARCHAR(20) NOT NULL,
    -- created | merged | duplicate_of | spawned_subreq
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON intake_to_req_links (intake_id);
CREATE INDEX ON intake_to_req_links (hu_id) WHERE hu_id IS NOT NULL;
CREATE INDEX ON intake_to_req_links (req_id) WHERE req_id IS NOT NULL;
```

## MCP tools

| tool | input | output |
|------|-------|--------|
| `domain_intake_submit` | `{source, raw_text?, raw_payload?, attachments?[], idempotency_key?}` | `{intake_id, status, async_processing: bool}` |
| `domain_intake_get_review` | `{intake_id}` | `{intake_payload, dedup_candidates, proposed_hu_draft, preview_md, suggested_actions[]}` |
| `domain_intake_approve` | `{intake_id, action: "create_new"\|"merge", target_hu_id?, edits?}` | `{resulting_req_id, resulting_hu_id, audit_log_id}` |
| `domain_intake_reject` | `{intake_id, reason, note?}` | `{ok: true}` |
| `domain_intake_list` | `{status?, source?, since?, limit?}` | `{intakes[], page_token?}` |
| `domain_intake_retry` | `{intake_id, from_step?}` | `{intake_id, status: "processing"}` |

## Pipeline implementation

- **Single Go service** `internal/sdd/intake/` con state machine deterministic.
- Cada step es un método del Service que persiste output en su columna correspondiente — NO RAM-only.
- Worker pool con `select` sobre `LISTEN intake_processing` (Postgres LISTEN/NOTIFY) + tick cada 30s para recovery.
- Skills builtin `intake.classify` y `intake.structure` reusan REQ-06 (LLM provider abstraction) y REQ-05 (skill registry).
- Adjuntos manejados vía REQ-04.6 S3 storage.

## Análisis breve

- **Qué pide:** receptor único de requerimientos heterogéneos con clasificación, dedup y estructuración por IA + review humano + commit transaccional + audit completo
- **Módulos sospechados:** `internal/sdd/intake/{service,steps,workers}.go` + `internal/store/pg/intake/` + skill builtins en `internal/skills/builtin/intake/`
- **Dependencias hard:** HU-04.1 (REQ/HU CRUD), HU-04.2 (Gherkin), HU-04.6 (S3), HU-04.9 (Jira sync — disparada por evento, no bloqueante), REQ-06 (LLM), REQ-05 (skills), REQ-03 (memory para dedup embeddings), REQ-10.3 (event bus)
- **Adapters deferred:** HU-04.8a email, HU-04.8b webhook, HU-04.8c sheets, HU-04.8d slack (futuras, declaran interface aquí)
- **Riesgos:** LLM hallucina gherkin que parece bueno pero no testea lo correcto → review humano obligatorio; dedup falso-negativo crea duplicados que limpiar después
- **Esfuerzo tentativo:** XL
