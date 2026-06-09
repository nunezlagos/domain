# Proposal: HU-04.8-intake-pipeline

## Intención

Pipeline única de ingestión de requerimientos desde fuentes heterogéneas (agente IA inicialmente, email/webhook/sheets/slack como adapters futuros), con clasificación IA, deduplicación semántica, generación de Gherkin draft y review humano obligatorio antes de impactar `requirements` / `user_stories` en BD. La pipeline es asincrónica, resumable y multi-tenant.

## Scope

**Incluye:**
- Tabla `intake_payloads` con state machine (received → processing → pending_review → committed|rejected|error)
- Tabla `intake_to_req_links` para trazabilidad (intake → REQ/HU resultante, link types: created/merged/duplicate_of/spawned_subreq)
- Steps deterministas: ingest, classify, dedupe, structure, review, commit
- 6 MCP tools: submit, get_review, approve, reject, list, retry
- Worker pool con LISTEN/NOTIFY + tick recovery
- Heartbeat por step + take-over de intakes huérfanos (>5min sin heartbeat)
- Idempotencia vía `idempotency_key` unique por org
- Skills builtin `intake.classify` y `intake.structure` (reusables y versionables)
- Validación de payload (size, mime, URLs no-SSRF)
- S3 staging bucket para attachments con lifecycle purge
- Audit log de todas las decisiones humanas (approve/reject/edits diff)
- Eventos al event bus: `intake.received`, `intake.ready_for_review`, `intake.committed`, `intake.rejected`
- Multi-tenant: scope por organization_id, RBAC sobre actions
- Quota soft/hard por org
- Métricas Prometheus + tracing OTel + structured logs
- Interfaz `IntakeSubmitter` que los adapters futuros implementarán

**No incluye:**
- Adapter de email (HU-04.8a futura)
- Adapter de webhook HTTP genérico (HU-04.8b futura)
- Adapter de Excel/Google Sheets (HU-04.8c futura)
- Adapter de Slack (HU-04.8d futura)
- Sync hacia Jira/GitHub/Linear (HU-04.9)
- State tracking unificado por entidad (HU-04.10)
- Web UI para review (parte de REQ-16)
- Bulk operations sobre múltiples intakes (post-MVP)

## Alternativas consideradas

### A. Pipeline síncrona (todo en submit, no async)

**Por qué no:** clasificar + dedupear + estructurar requiere 3-5s de llamadas LLM. Bloquear el submit es mala UX. Además, el storage de attachments grandes (5-10MB) puede tardar segundos. Async permite responder rápido y procesar en background.

### B. Sin state persistido (in-memory state machine)

**Por qué no:** rompible. Si el worker se cae con N intakes en vuelo, se pierden. Persistir cada step output en columna dedicada es trivial en Postgres y da resumability gratis.

### C. Queue externa (NATS/Redis/RabbitMQ)

**Por qué no en MVP:** complejidad operacional sin beneficio claro para los volúmenes esperados (<10k intakes/día). LISTEN/NOTIFY de Postgres + tick recovery es suficiente. Migrar después si se necesita.

### D. Sin review humano (auto-commit a confidence > X)

**Por qué no:** LLM puede generar Gherkin que suena bien pero testea lo equivocado. Hasta tener métricas de calidad reales del clasificador, **toda HU pasa por humano**. Modo `autopilot` (auto-approve sobre umbral) se considerará en HU futura con safeguards.

### E. Crear directamente en Jira sin pasar por BD interna

**Por qué no:** se pierde auditoría, trazabilidad SDD, multi-tenant scoping, dedup semántico contra el catálogo interno. Domain debe ser source of truth (decisión confirmada con el usuario). Jira es mirror downstream (HU-04.9).

### F. State machine declarativa (YAML config) vs hardcoded en Go

**Por qué no MVP:** YAML config (estilo REQ-09 flow-engine) suena flexible pero overkill para 4 steps fijos. Si la pipeline crece (10+ steps o variantes por source), considerar reusar REQ-09 flows en una HU futura.

## Dependencias

**Hard (deben existir antes):**
- HU-04.1 — requirements CRUD
- HU-04.2 — user-stories + Gherkin storage
- HU-04.6 — S3 attachments
- REQ-01 — DB schema base
- REQ-02.1, 02.2 — API keys + RBAC
- REQ-02.4 — audit log
- REQ-06.1, 06.2 — LLM provider abstraction y runners
- REQ-06.5 — embeddings pgvector (para dedup)
- REQ-05.1 — skill definitions (para builtin classify/structure)

**Soft (se integran si están, opcional):**
- REQ-10.3 — event bus (sin esto, los eventos son no-ops loggeados)
- REQ-17 — observability (sin esto, sólo logs locales)
- REQ-20.2/20.3 — notifications email/slack (sin esto, no hay notif al humano cuando pending_review)
- HU-04.9 — external sync (intake.committed dispara sync; sin 04.9 el evento queda sin consumer)
- HU-04.10 — state tracking unificado (intake state se ve solo en su tabla)

## Plan de release

1. Schema + store CRUD básico (sin pipeline)
2. Submit + ingest step (sin LLM, sin dedup, manual approve a través de SQL)
3. Classify + structure (LLM end-to-end manual)
4. Dedupe (embeddings)
5. Worker pool + LISTEN/NOTIFY + recovery
6. MCP tools approve/reject/list
7. Eventos + notifications
8. Métricas + traces
9. Quotas + abuse protection

## Riesgos

| Riesgo | Mitigación |
|---|---|
| LLM hallucina gherkin sin sentido | Review humano obligatorio; few-shot prompt; modelo con buena tasa en estructurar texto (Sonnet/Opus para structure, Haiku para classify) |
| Dedupe falso-negativo crea HUs redundantes | Merge en review humano; UI de "ver dupes encontrados"; threshold ajustable |
| Worker se cuelga en LLM call | Timeout por step; heartbeat; take-over por otro worker |
| Attachments huge devoran S3 | Tamaño máx 10MB; lifecycle purge bucket staging |
| LLM cost runaway | Métrica `intake_llm_cost_total{model}`; soft cap por org; alerta cost > X (REQ-15) |
| Sources sin idempotency key → dupes | Hash sintético desde campos disponibles (from+subject+sent_at o body+source); doc en adapters HUs |

## Tests críticos

- E2E: agent submits → polls → approves → row en user_stories existe + attachments referenciadas correctamente
- E2E: crash mid-pipeline → resume desde el último step persistido
- E2E: dedupe encuentra match → human merge → no se crea HU duplicada
- Sabotaje: payload 50MB → 413
- Sabotaje: 2 submits idénticos con misma idempotency_key → 1 sola row
- Sabotaje: 10 intakes paralelos → 0 race conditions en take-over
- Sabotaje: LLM timeout 60s → step marca error + retry funciona
