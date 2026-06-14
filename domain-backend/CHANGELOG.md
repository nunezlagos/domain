# Changelog

Todos los cambios notables a este proyecto se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/) y este proyecto adhiere a [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Commits siguen [Conventional Commits](https://www.conventionalcommits.org/) según `.claude/rules/git.md`.

## [Unreleased]

### Added — issue-08.10 sdd-pipeline-orchestrator implementación end-to-end — 2026-06-10 (sesión nocturna)

Implementación completa del orquestador SDD plug-and-play declarado en RFC 0006. 11 commits acumulados desde el foundation (28fddeb) hasta el último cierre. El binario `domain-mcp` ahora expone 4 herramientas MCP que cubren el pipeline SDD de extremo a extremo para los modos Express, Full y Detect.

**Schema + seeders**:
- Migración 000075 + catálogo v3 `agent_templates` (1 orchestrator + 10 phase-workers)
- Seeder nuevo `internal/seeds/flows_catalog.go` con `flow:sdd-pipeline-v1` (spec JSONB de 10 steps)

**Runner**:
- `RunOption` variadic en `internal/runner/agent`: `WithStandalone`, `WithFlowRun`, `WithFlowRunStep`
- `Runner.Env` + `checkOrphanPolicy`: en `prod`, runs sin `flow_run_id` requieren `WithStandalone` explícito (`ErrOrphanRunNotAllowed`)
- `buildRunMetadata` centralizado: `{}` cuando run es orquestado, `{"standalone":true,"reason":...}` cuando es directo

**Service orchestrator** (`internal/service/orchestrator/`):
- 5 modos declarados (`Express`, `Full`, `Solo`, `Detect`, `Async`); 3 implementados (Express, Full, Detect)
- 10 handlers de fase (`phases/sdd_*.go`): explore, spec, propose, design (D5 ADR required), tasks, apply (D5 code_reference required), verify, judge (D5 sabotage_record required), archive, onboard
- `Repository` interface + `pgRepository`: lookup `flow_id` por slug, CRUD `flow_runs` + `flow_run_steps`, mark step blocked/pending/completed/failed, update inputs, lookup `agent_templates.system_prompt` (BD source-of-truth)
- `Service.Run`: Express dispatcher pre-arma 2 prompts; Full dispatcher lazy-build (sólo step[0] hidratado, resto se reconstruye en `RecordPhaseResult` con `PriorOutputs` reales); Detect arma plan completo SIN persistir
- `Service.RecordPhaseResult`: valida D5 + handler.Validate, marca step completed/failed, propaga flow_run status agregado, rebuild prompt del próximo step
- `Service.ConfirmContinue`: D1 confirm condicional Express (files>1 OR lines>ExpressMaxLines → verify blocked)
- `Service.GetFlowStatus`: lectura de flow + steps con outputs y preview de user_prompt
- `Service.Clock` inyectable + `Service.Metrics` opcional
- `Service.hydrateSystemPrompts`: lookup vía `Repository.GetAgentTemplateSystemPrompt` por slug (cache local)

**MCP tools** (`internal/mcp/server/orchestrate_tools.go`):
- `domain_orchestrate` (raw_text, mode?, starting_phase?, skip_phases?, express_max_lines?)
- `domain_orchestrate_phase_result` (flow_run_step_id, output, memory_refs_saved?, duration_ms?)
- `domain_orchestrate_confirm` (flow_run_id, confirmed)
- `domain_flow_status` (flow_run_id)
- Wire-up `cmd/domain-mcp/main.go` con `phases.Registry.MustRegister` para los 10 handlers + `orchestrator.New(pool, recorder, registry, cfg.Env)` inyectado a `Deps.Orchestrator` + `agentRunnerInst.Env = cfg.Env`

**PromptRouter integration** (`internal/service/promptrouter/router.go`):
- `Router.Orchestrator` opcional: cuando se inyecta, los intents feat/refactor/doc/rfc invocan `Mode=Full`, fix/hotfix invocan `Mode=Express`; chat/idea siguen el bypass directo
- Firma `Route` extendida con `orgID` (requerido si Orchestrator!=nil)
- Nuevo `OutcomeOrchestratorStarted` con `FlowRunID + OrchestratorRunID + SnapshotPrompt`
- HTTP handler (`internal/api/handler/prompt_router.go`) + MCP tool `prompt_tools.go` propagan orgID desde Principal

**CLI**:
- `domain workflow resume <flow_run_id>`: tabla de steps + preview prompt del próximo pending (`cmd/domain/init_cli.go::runWorkflowResume`)

**Métricas Prometheus** (`internal/metrics/metrics.go`):
- `domain_orchestrator_runs_total{mode, status}` counter (status: started|completed|failed)
- `domain_orchestrator_phase_duration_seconds{phase, mode}` histogram
- `domain_orchestrator_phase_results_total{phase, mode, result}` counter
- `domain_orchestrator_confirms_total{confirmed}` counter
- `domain_orchestrator_required_save_missing_total{phase, save_type}` counter

**Convenciones consolidadas**:
- Lenguaje: español neutro project-wide (NO rioplatense)
- System prompts en BD (`agent_templates.system_prompt`), NO hardcoded en Go
- D5 enforcement bloqueante a nivel BD (sticky failure: step `failed` NO se re-marca)
- Repository abstrae persistencia: nil → in-memory fallback para tests unit

**Tests**:
- 633 unit verdes con `-short` (acumulados +97 vs baseline al inicio de la sesión)
- ~28 integration tests con testcontainers (orchestrator, seeds, MCP server, prompt router, D1 confirm)
- Suite integration corre en ~45 s
- Cero regresiones acumuladas en los 11 commits

**Pendientes** (no bloquean uso del orquestador, registrados en `tasks.md`):
- `svc-005` Solo mode (server-side LLM directo)
- `svc-007` Async mode (flow_signals + worker)
- `skill-001..003` auto-skill D3
- `ana-001..004` intent analysis D7
- `obs-003` OTel spans + `obs-004` alerts Prometheus
- 14 tests E2E restantes
- `sab-001` + `sab-002` sabotajes faltantes
- `doc-001..003` (sdd-pipeline.md, diagrama Mermaid, GETTING_STARTED)
- `state-001..003` cierre formal a `implemented`

### Architecture — orquestador SDD (RFC 0006 + 0007) — 2026-06-10 (tarde)

Sesión completa de diseño + ejecución del patrón orquestador SDD plug-and-play inspirado en [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai), extendido con capacidades Domain-native (modo Async via `flow_signals`, auto-skill inyectado por fase, integración con crons user-defined REQ-10).

**Principio rector adoptado:** Domain server = state + LLM + memoria + skills. Cliente IDE = ejecutor real (bash, edit, test, commit, grep workspace).

#### RFC 0006 `sdd-pipeline-orchestrator` — `accepted`

11 puntos consolidados con decisiones D1-D7 cerradas:

- **D1** Modo Express: auto-apply si ≤10 líneas + single-file; confirma si supera
- **D2** Multi-concern: auto-split sólo si todos los concerns son single-file
- **D3** Auto-skill threshold: 0.6 default, configurable por fase via `agent_templates.metadata.skill_threshold`
- **D4** Crons → flows: project-scoped, `target_type='flow'` con `inputs JSONB`, NO pasa por PromptRouter
- **D5** `suggested_saves`: `required=true` sólo en ADRs (sdd-design), code_references (sdd-apply), sabotage_records (sdd-judge)
- **D6** Express + Async: NO compatibles. Async sólo en Full y Detect
- **D7** Intent `analysis`: scope org con `created_by` visible

Modelo: 5 modos (Express/Full/Solo/Detect-only/Async). 10 fases SDD alineadas (`sdd-{orchestrator,explore,spec,propose,design,tasks,apply,verify,judge,archive,onboard}`).

#### RFC 0007 `rename HU → issue` — `accepted + executed`

Ejecutado atómicamente (1401 archivos, commit `7dc81e3`):
- Schema BD (migration 000073): `user_stories→issues`, `hu_drafts→issue_drafts`, 6 columnas `hu_id→issue_id`, `committed_hu_id→committed_issue_id`, triggers + índices renombrados
- Code Go: `userstory/→issue/`, `hubuilder/→issuebuilder/`. Identifiers: `UserStory→Issue`, `HUBuilder→IssueBuilder`, regex `^HU-\d+\.\d+ → ^issue-\d+\.\d+`
- Specs: 223 directorios `HU-XX.Y→issue-XX.Y` + 210 archivos `hu.md→issue.md`. Formato `XX.Y` mantenido (ligado a REQ-XX)
- Sin backwards-compat (Domain local-only, primer deploy a cloud RDS ya estará con naming nuevo)

#### issue-08.11 `heartbeat-watcher-cron` — `implemented`

System cron que detecta `flow_run_steps` con `status='running'` y `last_heartbeat_at > timeout` (default 5min), marca `failed` con `error='heartbeat_timeout'`, dispara `saga_compensation_log` según `retry_policy`.

- `internal/scheduler/cron/system/heartbeat_watcher.go` con `FOR UPDATE SKIP LOCKED`
- Cierra `flow_runs.status='failed'` cuando todos los steps están terminales
- Saga events: `require-cleanup → cleanup_required`, `re-emit → reemit_eligible`, default → `auto_retry_eligible`
- Métricas: `domain_heartbeat_watcher_stuck_total{org_id,phase,reason}`, `_ticks_total{result}`
- Lock key 1006. Config: `DOMAIN_HEARTBEAT_WATCHER_{ENABLED,TIMEOUT_MINUTES,TICK_SECONDS}`
- Tests: 1 unit + 4 integration verdes

#### issue-08.12 `orphan-runs-audit-cron` — `implemented`

System cron diario que cuenta `agent_runs` orphan (sin `flow_run_id` y sin `metadata.standalone='true'`), agrega por org e incrementa `domain_agent_runs_orphan_total{org_id, reason='bypass_service_layer'}`. Es la auditoría del enforcement híbrido del orquestador (RFC 0006 ADR-2).

- Migration 000074: `CREATE TABLE system_state (key,value,updated_at)` + `ALTER TABLE agent_runs ADD metadata JSONB DEFAULT '{}'`
- `internal/scheduler/cron/system/orphan_runs_audit.go` con idempotencia via `system_state.last_ack_at`
- Tick 24h con primera ejecución inmediata al boot
- Lock key 1007. Config: `DOMAIN_ORPHAN_AUDIT_{ENABLED,SCHEDULE}`
- Tests: 4 integration verdes (detección, standalone-ignored, idempotency, sabotage)

#### issue-08.10 `sdd-pipeline-orchestrator` — `in_progress` (foundation)

Foundation commit con las piezas base. Service orchestrator completo queda para sesión dedicada ~6-8h.

- Migration 000075: `agent_templates.role` + `seed_managed` + `is_user_modified` + `seed_version` + UNIQUE INDEX parcial `WHERE role='orchestrator'`
- Catálogo v3 en `internal/seeds/agent_templates_catalog.go`: replace 10 legacy por 11 `sdd-*` (1 orquestador + 10 phase-workers) con metadata `{phase, retry_policy, skill_threshold, required_saves}`
- Seeder v3 con `RETURNING (xmax = 0)` para distinguir Created/Updated correctamente (fix bug pre-existente)
- Cleanup defensivo respetando `is_user_modified=true`
- `Report.Deleted` nuevo en `internal/seeds/seeds.go`
- `runner.agent` marca `agent_runs.metadata.standalone=true` en direct_invocation (preserva compat con cron orphan-audit)
- Tests: 5 integration verdes

**Pendiente:** service `orchestrator/` con 5 modos · 10 phase handlers · seeder flow `sdd-pipeline-v1` · `WithStandalone` Option · 4 MCP tools `domain_orchestrate*` · CLI `workflow resume` · intent `analysis` · multi-concern detection · `suggested_saves` enforcement · 15 tests E2E · diagramas.

### Misc 2026-06-10 (tarde)

- **3 RFCs nuevos** en `docs/rfc/`: 0006, 0007 (ambos accepted)
- **3 specs nuevos** bajo `openspec/changes/REQ-08-agent-system/`: `issue-08.10` (in_progress), `issue-08.11` (implemented), `issue-08.12` (implemented)
- **Lock keys** agregados: 1006, 1007
- **Métricas Prometheus** registradas: 4 nuevas counters
- **Suite E2E verde** post-todo (14 tests, ~60s)
- **4 fallos pre-existentes documentados** (NO regresiones): `knowledge` build (`failingEmbedder` missing `EmbedBatch`), `audit` sabotage test, `migrate` hardcoded v45, `intake` 3 tests

---

### Added — plug-and-play flow (2026-06-10)

Wire-up real del flow plug-and-play en el binario `domain`:
- `cmd/domain/dev_bootstrap.go`: `domain dev-bootstrap` crea org + admin
  user + emite api_key + escribe `.env` para arranque dev en 1 comando.
- `cmd/domain/init_cli.go`: `domain init` detecta archivos `.md` de IA
  (CLAUDE.md, .claude/**, .opencode/**, .cursor/**, .windsurfrules,
  AGENTS.md, .cursorrules, .aider.conf.yml) + backup en BD + reemplaza
  por stubs apuntando al MCP. `domain workflow {list,restore}` cierra
  el ciclo de rollback.
- `cmd/domain/main.go` wire de `PromptRouter` + `Analyzer` (4 sources) +
  `LLMClassifier` (con fallback heurístico) + `AdaptiveService` +
  `WorkflowImport` en Deps del MCP server + `handler.API`. Antes
  estaban definidos pero no instanciados en runtime.
- `cmd/domain-mcp/main.go` espejo del wire-up para el binario stdio MCP.
- Endpoint HTTP `POST /api/v1/prompt` alternativo al MCP tool — útil para
  clientes no-MCP (web UI, scripts, curl, tests E2E HTTP reales).

Setup wizard ahora soporta `--auto-init` y `--skip-init`. Con
`--auto-init`, tras configurar Claude Desktop, automáticamente corre
`domain init` sobre el repo current y reemplaza los `.md` de IA por
stubs que apuntan al MCP.

### Added — wizard adaptive (HU-04.7 v2)

Reemplaza el flow de 8 preguntas fijas por análisis 4-fuentes + planner:
- `internal/service/wizardplan/` con `ContextEnvelope` + `Analyzer` +
  `Planner` + 4 sources (memory, hu_dedup FTS spanish, codebase grep,
  agent_runs history).
- `LLMQuestionFormulator` formula preguntas naturales contextualizadas
  con el envelope (Claude Haiku); fallback a templates determinísticos
  si no hay API key.
- `internal/service/hubuilder/AdaptiveService` envuelve el Service v1
  y solo pregunta los slots no inferidos.
- En promedio: **3-5 preguntas vs 8 fijas** del v1.

### Added — auditoría BD + tests por intent

- Migration `000072_grant_all_tables`: barrido idempotente de GRANTs a
  todas las tablas + sequences + views existentes + future-proof via
  `ALTER DEFAULT PRIVILEGES` sin role-target. Fix de bug crítico que
  hacía 38 tablas invisibles a `app_user` cuando las migrations corrían
  como `test` (testcontainers).
- `tests/e2e/schema_audit_test.go`: 3 tests que verifican 72 tablas
  críticas + counts de seeders + 13 FKs core.
- `tests/e2e/issue_types_test.go`: 10 tests E2E cubriendo TODOS los
  intent (chat, idea, feature, fix, hotfix, refactor, doc, rfc) + HU
  dedup + full happy path + sabotaje.
- `docs/flows/`: 9 diagramas Mermaid de secuencia (1 index + 8 por
  intent type) con asserts SQL post-flow.
- `docs/GETTING_STARTED.md`: quickstart 5-min plug-and-play.

### Changed — seeders coherentes con open-source sin cobro

`PlansSeeder` v2: slugs neutros (trial/standard/extended/unlimited) con
`monthly_price_usd = 0` hardcoded. Cleanup defensivo de legacy slugs
comerciales (free/pro/starter/team/enterprise) que NO estén asignados a
ninguna org. Decisión HU-21.4 archived: Domain open-source sin cobro.

Nuevos catálogos (seeders globales):
- `ModelRegistrySeeder`: 15 models (Claude 4.x, GPT-4o/5, Gemini, Voyage,
  Ollama) con pricing USD por 1M tokens + context_size + modality.
- `PlatformPoliciesSeeder`: 10 policies baseline (TDD strict, RLS
  defense-in-depth, conventional commits, low-cardinality metrics, etc.).

Per-org (helpers `SeedXForOrg`):
- `SkillCatalog`: 7 skills built-in (intake-classify, intake-structure,
  code-search, file-read, web-fetch, summarize, extract-entities).
- `AgentTemplateCatalog`: 10 templates (researcher, coder, reviewer,
  tester, supervisor, doc-writer, sdd-spec-writer, security-auditor,
  intake-triager, general-assistant) con system_prompt + personality +
  capabilities + model + handoff_policy.

### Fixed — migrations

- 000038 duplicate file (renombrado a 000070).
- 000063 flow_steps_heartbeat ahora CREATE TABLE IF NOT EXISTS antes
  del ALTER (tabla `flow_run_steps` faltaba).
- 000070 cost_view_indexes sin CONCURRENTLY (golang-migrate usa tx).
- 000072 grants barridos globales (ver Added arriba).

### Implemented (estado snapshot a 2026-06-10)

**139/139 HUs activas implementadas (100%)** — 3 archived
(HU-21.4 Stripe, HU-25.5 dup, HU-16.3 web-flow-editor por decisión
db-first).

REQs cerrados al 100%: REQ-02, 03, 04, 12, 14, 15, 17, 19, 20, 22, 27.

Builds: 5/5 E2E tests verdes (38s). Schema: 78 tablas en BD post-migrate.

### Implemented (Fase 0 + parte Fase 1)
- HU-01.6 local-dev-environment — docker-compose con Postgres+pgvector, MinIO, Adminer, Mailpit
- HU-01.1 db-schema-migrations — 23 migraciones SQL con golang-migrate embebido + 7 tests integration testcontainers
- HU-01.2 config-system — load env DOMAIN_* + Validate strict + unit tests
- HU-01.3 health-version — HTTP /health + /health/ready + ldflags version inject + unit tests

### Added
- docs/testing-workflow.md — flujo TDD obligatorio per HU
- Spec inicial: 27 REQs, 148 HUs en `openspec/changes/`
- 5 RFCs de boundaries arquitectónicas en `docs/rfc/`
- 9 reglas de conventions en `.claude/rules/`
- Roadmap detallado con 6 fases en `docs/roadmap.md`
- Sistema de policies en BD (HU-01.8) — DB como source of truth
- Seeders Go embebidos (HU-01.7) para catálogos iniciales
- MCP tool resilience strict (HU-12.6) con timeout + CB + cache LRU
- DB tooling + hardening (REQ-25, 13 HUs): PgBouncer, RLS, pgaudit, read replicas, password rotation, anonymization, etc.
- Horizontal scalability patterns (REQ-26, 7 HUs)
- Vertical performance tuning (REQ-27, 4 HUs)

### Changed
- HU-02.7 reescrita de Google OAuth a passwordless OTP por email con RUT/email identifier

### Notes
- Status del backlog: 100% `proposed`, 0 HUs implementadas
- Próximo paso: kickoff Fase 0 (bootstrap dev environment) según `docs/roadmap.md`

---

## Plantilla para futuros releases

```markdown
## [vX.Y.Z] - YYYY-MM-DD

### Added
- Nueva feature backwards-compatible (commits `feat:`)

### Changed
- Cambio en comportamiento existente (commits `refactor:` con impacto visible)

### Deprecated
- Features que se removerán en próximos releases

### Removed
- Features eliminadas en este release (commits `feat!:` con removal)

### Fixed
- Bug fixes (commits `fix:`)

### Security
- Patches de seguridad
```

[Unreleased]: https://github.com/saargo/domain/compare/v0.0.0...HEAD
