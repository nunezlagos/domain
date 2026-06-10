# Changelog

Todos los cambios notables a este proyecto se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/) y este proyecto adhiere a [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Commits siguen [Conventional Commits](https://www.conventionalcommits.org/) según `.claude/rules/git.md`.

## [Unreleased]

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
