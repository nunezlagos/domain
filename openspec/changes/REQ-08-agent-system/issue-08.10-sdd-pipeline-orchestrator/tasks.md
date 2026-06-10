# Tasks: issue-08.10-sdd-pipeline-orchestrator

## Schema (1 migration aditiva)

- [ ] **mig-001**: `000074_agent_templates_role.up.sql` — ADD COLUMN role + CHECK + UNIQUE INDEX parcial (role='orchestrator')
- [ ] **mig-002**: `000074_agent_templates_role.down.sql`

## Seeders

- [ ] **seed-001**: `agent_templates_catalog.go` v3 — replace 10 entries: 1×sdd-orchestrator (role='orchestrator') + 9×sdd-{explore,spec,propose,design,tasks,apply,verify,judge,archive,onboard}
- [ ] **seed-002**: Cleanup defensivo (mismo patrón PlansSeeder v2)
- [x] **seed-003**: `internal/seeds/flows_catalog.go` — seeder `flow:sdd-pipeline-v1` per-org con spec JSONB DAG 10 steps — 2026-06-10 (idempotente UPSERT + cleanup defensivo respeta is_user_modified + flow_runs activos)

## Service `internal/service/orchestrator/`

- [x] **svc-001**: `service.go` — Service + Run(ctx, OrchestrateInput) (orchestrator_run_id, flow_run_id, error) — skeleton 2026-06-10
- [x] **svc-002**: `OrchestrateInput` struct (Mode, RawText, StartingPhase, SkipPhases, AsyncTimeout, ExpressMaxLines) — 2026-06-10
- [x] **svc-003**: `modes/express.go` — sub-S fast path (sdd-apply + sdd-verify only) — 2026-06-10 (in-memory; persistencia flow_runs pendiente)
- [ ] **svc-004**: `modes/full.go` — pipeline 10 fases vía sub-agents nativos del IDE
- [ ] **svc-005**: `modes/solo.go` — inline execution server-side con LLM provider directo
- [ ] **svc-006**: `modes/detect.go` — dry_run=true, persiste a `*.status='draft'`
- [ ] **svc-007**: `modes/async.go` — emite flow_signals, reanuda con worker que tail
- [~] **svc-008**: `modes/validator.go` — validate `async + express` → ErrAsyncModeUnsupported (D6) — validate() en service.go cubre D6 + empty/mode/unknown-phase; falta DAG-check de SkipPhases
- [ ] **svc-009**: `phases/sdd_explore.go` — system_prompt + input/output schema + multi-concern detection (D2)
- [ ] **svc-010**: `phases/sdd_spec.go` — delega a issuebuilder.AdaptiveService (issue-04.7)
- [ ] **svc-011**: `phases/sdd_propose.go`
- [ ] **svc-012**: `phases/sdd_design.go` — suggested_saves required=true para ADRs (D5)
- [ ] **svc-013**: `phases/sdd_tasks.go`
- [x] **svc-014**: `phases/sdd_apply.go` — code_reference D5 required + RetryCleanup + multi_concern/blocked detection — 2026-06-10 (D1 confirm condicional pendiente para wire-up MCP)
- [x] **svc-015**: `phases/sdd_verify.go` — Gherkin verifier + RetryReemit + blocker/failed scenarios — 2026-06-10
- [ ] **svc-016**: `phases/sdd_judge.go` — suggested_saves required=true para sabotage_records (D5)
- [ ] **svc-017**: `phases/sdd_archive.go` — entity_state_transitions to_state='archived'
- [ ] **svc-018**: `phases/sdd_onboard.go` — opcional, genera knowledge_doc si aplica
- [x] **svc-019**: `phases/registry.go` — map slug → handler + retry_policy lookup — 2026-06-10 (Handler iface + Registry concurrent-safe + SuggestedSave/RetryPolicy/MemoryRef)

## Service enforcement

- [x] **enf-001**: `internal/runner/agent/options.go` — Option pattern + WithStandalone(bool) + WithFlowRun(uuid) + WithFlowRunStep — 2026-06-10 (movido a `runner/agent/` que es donde se crean los agent_runs)
- [x] **enf-002**: `internal/runner/agent/runner.go::Run` — checkOrphanPolicy: prod + flow_run_id nil + !standalone → ErrOrphanRunNotAllowed — 2026-06-10
- [x] **enf-003**: errores tipados — ErrOrphanRunNotAllowed en `runner/agent`, ErrAsyncModeUnsupported + ErrRequiredSaveMissing en `service/orchestrator/errors.go` — 2026-06-10

## Auto-skill integration (consume issue-05.4)

- [ ] **skill-001**: `internal/service/orchestrator/skills.go` — fetchRecommendedSkills(ctx, phaseContext, threshold) usa /api/skills/recommend
- [ ] **skill-002**: agent_templates.metadata.skill_threshold lookup per phase (D3)
- [ ] **skill-003**: response builder incluye `skills_recommended` array

## suggested_saves contract (D5)

- [x] **save-001**: SuggestedSave struct vive en `phases/registry.go`; buildSaves implícito en cada handler — 2026-06-10
- [x] **save-002**: `orchestrator/saves.go::ValidateRequiredSaves` con RequiredSaveError + Unwrap → ErrRequiredSaveMissing — 2026-06-10
- [~] **save-003**: Tests unit cubren sdd-apply (1 required code_reference) y sdd-verify (0 required); faltan sdd-design (3 required ADRs), sdd-judge (sabotage_record) cuando se implementen los handlers

## Métricas + observabilidad

- [ ] **obs-001**: `internal/metrics/orchestrator.go` — phase_duration_seconds histogram + runs_total counter
- [ ] **obs-002**: `internal/metrics/agent.go` — agent_runs_orphan_total counter (org_id, reason)
- [ ] **obs-003**: OTel span por fase, attribute `flow_run_step.id` (vía SafeAttrs de issue-17.2)
- [ ] **obs-004**: `deploy/prometheus/alerts/orchestrator.yml` — alerts orphan_runs > 0 por 5min

## MCP tools nuevos

- [ ] **mcp-001**: `internal/mcp/tools/orchestrate.go::domain_orchestrate` (raw_text, mode?, starting_phase?, skip_phases?)
- [ ] **mcp-002**: `domain_orchestrate_phase_result` (flow_run_step_id, output, memory_refs_saved)
- [ ] **mcp-003**: `domain_orchestrate_confirm` (flow_run_id, confirmed bool) — para D1 confirm condicional
- [ ] **mcp-004**: `domain_flow_status` (flow_run_id? optional) — list flows activos del user
- [ ] **mcp-005**: Wire-up en `cmd/domain-mcp/main.go` y `cmd/domain/main.go::runServer`
- [ ] **mcp-006**: PromptRouter integration — feat/fix/refactor/hotfix/rfc/doc invokan orchestratorSvc.Run() (NO chat, NO idea, NO analysis)

## CLI

- [ ] **cli-001**: `cmd/domain/workflow_resume.go` — `domain workflow resume <flow_run_id>` que devuelve last snapshot + next prompt

## Intent analysis (D7)

- [ ] **ana-001**: PromptRouter clasifica `analysis` como intent separado
- [ ] **ana-002**: `internal/service/orchestrator/analysis/` — mini-pipeline 2 fases (explore + write_doc)
- [ ] **ana-003**: Genera knowledge_doc con source='analysis', created_by, scope=org
- [ ] **ana-004**: Crea observation indexable apuntando al doc

## Tests E2E (1 por escenario del issue.md)

- [ ] **test-001**: Re-cataloging (escenario 1)
- [ ] **test-002**: UNIQUE INDEX orchestrator único (escenario 2)
- [ ] **test-003**: Modo Express con confirm condicional D1 (escenario 3)
- [ ] **test-004**: Multi-concern auto-split D2 (escenario 4)
- [ ] **test-005**: State server + execution client (escenario 5)
- [ ] **test-006**: Resume cross-session (escenario 6)
- [ ] **test-007**: Dual output (escenario 7)
- [ ] **test-008**: Auto-skill threshold D3 (escenario 8)
- [ ] **test-009**: Cron → flow D4 (escenario 9)
- [ ] **test-010**: suggested_saves required D5 (escenario 10)
- [ ] **test-011**: Async D6 (escenario 11)
- [ ] **test-012**: Intent analysis D7 (escenario 12)
- [ ] **test-013**: Service-layer enforcement orphan (escenario 13)
- [ ] **test-014**: Sabotage INSERT bypass (escenario 14)
- [ ] **test-015**: Recovery desde snapshot (escenario 15)

## Sabotaje

- [ ] **sab-001**: INSERT directo bypass → métrica orphan incrementa dentro 5min vía cron issue-08.12
- [ ] **sab-002**: Intentar 2 templates con role='orchestrator' por org → UNIQUE violation
- [x] **sab-003**: Forzar required_save missing → fase no avanza, error específico — 2026-06-10 (`TestService_Sabotage_ApplyMissingRequiredCodeReference`)

## Docs

- [ ] **doc-001**: `docs/agents/sdd-pipeline.md` — descripción flow + 5 modos con ejemplos
- [ ] **doc-002**: `docs/flows/09-orchestrator.md` — diagrama Mermaid (sumar al set existente)
- [ ] **doc-003**: Actualizar `docs/GETTING_STARTED.md` con sección "Primer prompt con orquestador"
- [ ] **doc-004**: `CHANGELOG.md` Unreleased — agregar entrada issue-08.10

## Estado

- [ ] **state-001**: Actualizar state.yaml a `implemented` post-merge
- [ ] **state-002**: Actualizar RFC 0006 con link a issue-08.10 implementado
- [ ] **state-003**: Actualizar `openspec/changes/REQ-08-agent-system/req.md` con HU-08.10 implementado
