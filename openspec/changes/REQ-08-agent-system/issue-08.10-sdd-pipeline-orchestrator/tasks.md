# Tasks: issue-08.10-sdd-pipeline-orchestrator

## Schema (1 migration aditiva)

- [ ] **mig-001**: `000074_agent_templates_role.up.sql` — ADD COLUMN role + CHECK + UNIQUE INDEX parcial (role='orchestrator')
- [ ] **mig-002**: `000074_agent_templates_role.down.sql`

## Seeders

- [ ] **seed-001**: `agent_templates_catalog.go` v3 — replace 10 entries: 1×sdd-orchestrator (role='orchestrator') + 9×sdd-{explore,spec,propose,design,tasks,apply,verify,judge,archive,onboard}
- [ ] **seed-002**: Cleanup defensivo (mismo patrón PlansSeeder v2)
- [ ] **seed-003**: NUEVO `flows_catalog.go` — seeder de `flow:sdd-pipeline-v1` per-org con spec JSONB DAG 10 steps

## Service `internal/service/orchestrator/`

- [ ] **svc-001**: `service.go` — Service + Run(ctx, OrchestrateInput) (orchestrator_run_id, flow_run_id, error)
- [ ] **svc-002**: `OrchestrateInput` struct (Mode, RawText, StartingPhase, SkipPhases, AsyncTimeout, ExpressMaxLines)
- [ ] **svc-003**: `modes/express.go` — sub-S fast path (sdd-apply + sdd-verify only)
- [ ] **svc-004**: `modes/full.go` — pipeline 10 fases vía sub-agents nativos del IDE
- [ ] **svc-005**: `modes/solo.go` — inline execution server-side con LLM provider directo
- [ ] **svc-006**: `modes/detect.go` — dry_run=true, persiste a `*.status='draft'`
- [ ] **svc-007**: `modes/async.go` — emite flow_signals, reanuda con worker que tail
- [ ] **svc-008**: `modes/validator.go` — validate `async + express` → ErrAsyncModeUnsupported (D6)
- [ ] **svc-009**: `phases/sdd_explore.go` — system_prompt + input/output schema + multi-concern detection (D2)
- [ ] **svc-010**: `phases/sdd_spec.go` — delega a issuebuilder.AdaptiveService (issue-04.7)
- [ ] **svc-011**: `phases/sdd_propose.go`
- [ ] **svc-012**: `phases/sdd_design.go` — suggested_saves required=true para ADRs (D5)
- [ ] **svc-013**: `phases/sdd_tasks.go`
- [ ] **svc-014**: `phases/sdd_apply.go` — express confirm condicional D1, retry-policy require-cleanup
- [ ] **svc-015**: `phases/sdd_verify.go`
- [ ] **svc-016**: `phases/sdd_judge.go` — suggested_saves required=true para sabotage_records (D5)
- [ ] **svc-017**: `phases/sdd_archive.go` — entity_state_transitions to_state='archived'
- [ ] **svc-018**: `phases/sdd_onboard.go` — opcional, genera knowledge_doc si aplica
- [ ] **svc-019**: `phases/registry.go` — map slug → handler + retry_policy lookup

## Service enforcement

- [ ] **enf-001**: `internal/service/agent/option.go` — Option pattern + WithStandalone(bool) + WithFlowRun(uuid)
- [ ] **enf-002**: `internal/service/agent/service.go::Create` — if flow_run_id nil AND !standalone AND env='production' → ErrOrphanRunNotAllowed
- [ ] **enf-003**: `internal/service/agent/errors.go` — ErrOrphanRunNotAllowed + ErrAsyncModeUnsupported + ErrRequiredSaveMissing

## Auto-skill integration (consume issue-05.4)

- [ ] **skill-001**: `internal/service/orchestrator/skills.go` — fetchRecommendedSkills(ctx, phaseContext, threshold) usa /api/skills/recommend
- [ ] **skill-002**: agent_templates.metadata.skill_threshold lookup per phase (D3)
- [ ] **skill-003**: response builder incluye `skills_recommended` array

## suggested_saves contract (D5)

- [ ] **save-001**: `internal/service/orchestrator/saves.go` — SuggestedSave struct + buildSaves(phase) handler
- [ ] **save-002**: Validation post-phase: si required save no en cliente_memory_refs reportados → ErrRequiredSaveMissing
- [ ] **save-003**: Tests unit por fase: sdd-design emite 3 required, sdd-apply emite 1 required, etc.

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
- [ ] **sab-003**: Forzar required_save missing → fase no avanza, error específico

## Docs

- [ ] **doc-001**: `docs/agents/sdd-pipeline.md` — descripción flow + 5 modos con ejemplos
- [ ] **doc-002**: `docs/flows/09-orchestrator.md` — diagrama Mermaid (sumar al set existente)
- [ ] **doc-003**: Actualizar `docs/GETTING_STARTED.md` con sección "Primer prompt con orquestador"
- [ ] **doc-004**: `CHANGELOG.md` Unreleased — agregar entrada issue-08.10

## Estado

- [ ] **state-001**: Actualizar state.yaml a `implemented` post-merge
- [ ] **state-002**: Actualizar RFC 0006 con link a issue-08.10 implementado
- [ ] **state-003**: Actualizar `openspec/changes/REQ-08-agent-system/req.md` con HU-08.10 implementado
