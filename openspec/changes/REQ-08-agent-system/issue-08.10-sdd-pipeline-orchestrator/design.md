# Design: issue-08.10-sdd-pipeline-orchestrator

## Contexto

Ver RFC 0006 (`docs/rfc/0006-sdd-pipeline-orchestrator.md`, accepted 2026-06-10) para la motivación completa y decisiones D1-D7.

Resumen: hoy `agent_templates` es catálogo flat con slugs rol-genérico. Patrón gentle-ai demuestra que slug↔fase-SDD da mejor reproducibilidad. Domain extiende con modo Async + auto-skill + crons.

## Principio rector (del RFC)

**Domain server = state + LLM + memoria + skills.**
**Cliente IDE = ejecutor real** (bash, edit, test, commit, grep workspace).

Verificación: todos los MCP tools en `internal/mcp/tools/*.go` son data-only (cero `os.Open`, cero `exec.Command`, cero filesystem writes).

## ADRs

### ADR-1 — Cleanup defensivo en seeder (no soft-delete)

Decisión: Replace + cleanup en seeder. Patrón establecido en `PlansSeeder v2` (issue-21.3).

Justificación: Domain hoy es local-only sin orgs con `is_user_modified=true` reales. Soft-delete sería teatro. El cleanup defensivo respeta customizaciones (`WHERE is_user_modified=false`).

### ADR-2 — Enforcement híbrido reforzado

Decisión: service-layer + métrica + cron + sabotage test.

Alternativas:
- Hard CHECK `flow_run_id NOT NULL` → rompe debugging (rechazado)
- Soft warning → bypaseable con INSERT directo (rechazado)
- ✅ Híbrido: `WithStandalone(true)` flag explícito + métrica + cron + alert

Tradeoff: schema permite NULL pero service-layer + cron lo detectan. Visibility sin bloqueo dura.

### ADR-3 — 5 modos (Express agregado a los 4 del RFC original)

Decisión D1 del RFC: agregar **Express** como pre-fase del classifier.

| Modo | Cuándo | Trigger |
|---|---|---|
| Express | `estimated_scope='single-line' OR 'single-file'` | classifier auto-detect |
| Full | default para `multi-file` o `multi-module` | classifier |
| Solo | API/MCP sin sub-agent support | input flag |
| Detect | dry-run para CI/CD | input flag |
| Async | pause/resume cross-session | flow config |

**D6 enforcement:** Async sólo compatible con Full y Detect. Si input declara `mode='express' AND async=true` → error `async_mode_unsupported`.

### ADR-4 — State server / Execution client (corrige error del spec previo)

Pre-RFC el spec decía "server-side bias" para algunas fases (sdd-explore, sdd-design). Eso era **retroceso conceptual**: Domain no puede ejecutar grep sobre el workspace del cliente porque no tiene filesystem access. Las observations + knowledge_chunks son caché parcial, NO sustituyen el workspace fresh.

**Modelo correcto:**

| Responsabilidad | Server | Cliente IDE |
|---|:-:|:-:|
| Decidir fase siguiente | ✓ | |
| Formular prompt para la fase | ✓ | |
| Persistir state + snapshots | ✓ | |
| Inyectar skills recomendadas | ✓ | |
| Sugerir guardados de memoria | ✓ | |
| Ejecutar grep/read del workspace | | ✓ |
| Ejecutar bash/tests/git/edits | | ✓ |
| Llamar `domain_mem_save/search` | | ✓ |

### ADR-5 — D4: crons → flows pre-registrados, sin PromptRouter

Decisión: `crons.target_type='flow', target_id=<flow_uuid>, inputs JSONB`.

NO pasa por PromptRouter porque NO hay prompt natural. El flow está pre-definido con DAG + input schema. Cada project puede registrar flows reusables (ej. `weekly-security-audit`, `daily-cost-report`); `sdd-pipeline-v1` es UNO de esos flows.

Justificación: eficiencia (sin LLM call de classify) + predictibilidad (idempotente sobre el flow registrado). El scheduler existente con leader election (`internal/scheduler/`) ya soporta este patrón.

### ADR-6 — D5: suggested_saves con `required: true` en críticos

Defaults: `required: false` (cliente decide).

Marcados `required: true` SÓLO en:
- `sdd-design` → ADRs (entity_state_transitions registra la transición)
- `sdd-apply` → code_references (file_path + commit_sha post-commit)
- `sdd-judge` → sabotage_records

Si el cliente IDE NO ejecuta un required save:
- Fase no avanza
- Devuelve error `RequiredSaveMissing` con la lista de topics faltantes
- Cliente IDE debe ejecutar save + re-reportar phase_result

Justificación: preserva el modelo memory-explícito actual (cliente IDE decide) pero garantiza traceability de decisiones críticas que rotan en cross-fase. Sin esto, sdd-tasks no sabe los ADRs que sdd-design decidió.

### ADR-7 — Mapeo fase ↔ tabla SDD (verificado, cero schema nuevo)

```
sdd-explore   → intake_payloads (source=agent_orchestrator) + observations (search)
sdd-spec      → issue_drafts (delega al wizard adaptive issue-04.7)
sdd-propose   → proposals (status: draft → approved)
sdd-design    → designs (arch_decisions + tdd_plan + alternatives)
sdd-tasks     → tasks (descompone)
sdd-apply     → code_references + commit (vía cliente IDE)
sdd-verify    → verification_results
sdd-judge     → sabotage_records (TDD strict step 4)
sdd-archive   → entity_state_transitions (to_state='archived')
sdd-onboard   → knowledge_docs + platform_policies (opcional)
```

**Cero schema nuevo.** Sólo se agrega `agent_templates.role`.

### ADR-8 — MCP tools nuevos

| Tool | Quién lo llama | Qué hace |
|---|---|---|
| `domain_orchestrate` | cliente IDE (1 vez por prompt) | dispara flow_run + devuelve primera fase |
| `domain_orchestrate_phase_result` | cliente IDE (1 vez por fase) | reporta output de la fase + recibe siguiente |
| `domain_orchestrate_confirm` | cliente IDE (cuando D1 pide confirm) | OK explícito antes de sdd-apply commit |
| `domain_flow_status` | cliente IDE (al iniciar conversación) | lista flow_runs activos del user |

### ADR-9 — Retry policy explícita por phase (D del RFC)

Cada `agent_templates.metadata.retry_policy`:

| Política | Comportamiento | Defaults |
|---|---|---|
| `idempotent` | Re-corre desde 0, sobreescribe snapshot | sdd-explore, sdd-onboard |
| `re-emit` | Usa snapshot anterior, no re-LLM | sdd-archive |
| `require-cleanup` | Saga compensation antes del retry | sdd-apply (rollback commit), sdd-tasks (delete tasks) |

## Patrones aplicados

- **Strategy** — 5 modos como `orchestrator.Mode` interface
- **Saga** — `saga_compensation_log` existente para rollback
- **State machine** — `flow_runs.status` con paused_awaiting_*
- **Repository** — agent_templates accedido sólo via service
- **Adapter** — MCP tool `domain_orchestrate` adapta a `orchestratorSvc.Run()`
- **Pipeline** — `flow_run_steps` con position secuencial

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|---|---|---|
| Solo agota context con 10 fases inline | Media | Budget caps por fase + summary handoff via observations |
| Async pause indefinida | Media | `flow_runs.timeout_at` default 7d + alert si pausa > 3d |
| Loop orchestrator → orchestrator | Baja | `flow_run_steps.depth ≤ 1` (CHECK constraint) |
| Orphan cron false positives en dev | Alta | Cron sólo corre con `DOMAIN_ENV=production` |
| Cleanup borra template legacy con agent_runs activos | Media | `NOT EXISTS (running)` en WHERE; falla silenciosa, reintenta próximo seed |

## Observabilidad

- `domain_orchestrator_phase_duration_seconds{phase, mode}` histogram
- `domain_orchestrator_runs_total{mode, status}` counter
- `domain_agent_runs_orphan_total{org_id, reason}` counter (consumido por issue-08.12)
- OTel span por fase con `flow_run_step.id` como attribute (issue-17.2 SafeAttrs)
- Log Info en cada `flow_run_steps.status` transition (sin PII)
