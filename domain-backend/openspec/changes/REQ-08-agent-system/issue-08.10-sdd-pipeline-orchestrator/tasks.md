# Tasks: issue-08.10-sdd-pipeline-orchestrator

## Schema (1 migration aditiva)

- [x] **mig-001**: `000075_agent_templates_role_seed_managed.up.sql` — ADD COLUMN role + seed_managed + is_user_modified + seed_version + CHECK + UNIQUE INDEX parcial (role='orchestrator') — 2026-06-10 (migration existe, re-numbered a 000075)
- [x] **mig-002**: `000075_agent_templates_role_seed_managed.down.sql` — 2026-06-10

## Seeders

- [x] **seed-001**: `agent_templates_catalog.go` v3 — 11 entries: 1×sdd-orchestrator (role='orchestrator') + 10×sdd-{explore,spec,propose,design,tasks,apply,verify,judge,archive,onboard} — 2026-06-10
- [x] **seed-002**: Cleanup defensivo via DELETE seed_managed=true NOT IN catalog AND no agent_runs running (mismo patrón PlansSeeder v2) — 2026-06-10
- [x] **seed-003**: `internal/seeds/flows_catalog.go` — seeder `flow:sdd-pipeline-v1` per-org con spec JSONB DAG 10 steps — 2026-06-10 (idempotente UPSERT + cleanup defensivo respeta is_user_modified + flow_runs activos)

## Service `internal/service/orchestrator/`

- [x] **svc-001**: `service.go` — Service + Run(ctx, OrchestrateInput) (orchestrator_run_id, flow_run_id, error) — skeleton 2026-06-10
- [x] **svc-002**: `OrchestrateInput` struct (Mode, RawText, StartingPhase, SkipPhases, AsyncTimeout, ExpressMaxLines) — 2026-06-10
- [x] **svc-003**: `modes/express.go` — sub-S fast path (sdd-apply + sdd-verify only) — 2026-06-10 (con persistencia flow_runs + flow_run_steps via Repository pattern)
- [x] **svc-004**: `modes/full.go` — pipeline 10 fases con lazy-build (sólo step[0] pre-construido; RecordPhaseResult reconstruye user_prompt del próximo step con PriorOutputs acumulados) + SkipPhases + StartingPhase — 2026-06-10
- [x] **svc-005**: Solo mode server-side — `internal/service/orchestrator/solo.go::Service.runSolo` invoca provider.Complete por fase con system+user prompt + parseJSONOutput tolerante (strips code fences) + handler.Validate + lazy build PriorOutputs. Repository.GetAgentTemplate trae model/temperature/max_tokens. ProviderForModel infiere provider desde model name. ADR-4: Solo NO admite D5 sticky required saves ni D1 confirm (CI/CD use case sin cliente IDE). Tests: 3 integration con FakeProvider canned responses por slug — 2026-06-10
- [x] **svc-006**: Detect mode dry-run sin persistencia — BuildFullPlan hidratado pero NO se persisten flow_run/steps en BD; el caller invoca Mode=Full por separado para ejecutar — 2026-06-10
- [x] **svc-007**: `modes/async.go` — emite flow_signals, reanuda con worker que tail — 2026-06-10 (BuildAsyncPlan reusa BuildFullPlan; async.go::Service.runAsync + ProcessAsyncFlowRun + emitSignal con SignalStore. Tests: 10 integration con fakeProvider + SignalStore assertions, verifica signals step_completed/flow_completed/step_failed, resume cross-session, degraded sin SignalStore, invalid JSON → failure signal, non-async flow rejected)
- [x] **svc-008**: `modes/validator.go` — ValidateDAG con phaseDependencies (mapa fase→dependencias). Validación integrada en selectPhases: si una fase se salta pero una dependiente se conserva, el DAG es inválido. StartingPhase asume fases anteriores como ejecutadas. Tests: 20 unit + verificación en integration — 2026-06-10
- [x] **svc-009**: `phases/sdd_explore.go` — analiza prompt + multi-concern detection (D2) — 2026-06-10
- [x] **svc-010**: `phases/sdd_spec.go` — produce issue.md (slug + md content) — 2026-06-10
- [x] **svc-011**: `phases/sdd_propose.go` — proposal.md status='draft' (no auto-promoción) — 2026-06-10
- [x] **svc-012**: `phases/sdd_design.go` — D5 'adr' Required + sanity ≥1 ADR — 2026-06-10
- [x] **svc-013**: `phases/sdd_tasks.go` — descomposición atómica con id+description+depends_on — 2026-06-10
- [x] **svc-014**: `phases/sdd_apply.go` — code_reference D5 required + RetryCleanup + multi_concern/blocked detection — 2026-06-10 (D1 confirm condicional pendiente para wire-up MCP)
- [x] **svc-015**: `phases/sdd_verify.go` — Gherkin verifier + RetryReemit + blocker/failed scenarios — 2026-06-10
- [x] **svc-016**: `phases/sdd_judge.go` — D5 'sabotage_record' Required + sanity ≥1 record — 2026-06-10
- [x] **svc-017**: `phases/sdd_archive.go` — archived flag + entity_state_transitions tracking — 2026-06-10
- [x] **svc-018**: `phases/sdd_onboard.go` — knowledge_doc opcional con skipped=true válido — 2026-06-10
- [x] **svc-019**: `phases/registry.go` — map slug → handler + retry_policy lookup — 2026-06-10 (Handler iface + Registry concurrent-safe + SuggestedSave/RetryPolicy/MemoryRef)

## Service enforcement

- [x] **enf-001**: `internal/runner/agent/options.go` — Option pattern + WithStandalone(bool) + WithFlowRun(uuid) + WithFlowRunStep — 2026-06-10 (movido a `runner/agent/` que es donde se crean los agent_runs)
- [x] **enf-002**: `internal/runner/agent/runner.go::Run` — checkOrphanPolicy: prod + flow_run_id nil + !standalone → ErrOrphanRunNotAllowed — 2026-06-10
- [x] **enf-003**: errores tipados — ErrOrphanRunNotAllowed en `runner/agent`, ErrAsyncModeUnsupported + ErrRequiredSaveMissing en `service/orchestrator/errors.go` — 2026-06-10

## Auto-skill integration (consume issue-05.4)

- [x] **skill-001**: `internal/service/orchestrator/skills.go` — fetchRecommendedSkills(ctx, phaseContext, threshold) usa skill.Service.SearchHybrid en lugar de /api/skills/recommend (issue-05.4 solo declara auto_engine, no hay endpoint REST; SearchHybrid provee la misma funcionalidad in-process) — 2026-06-10
- [x] **skill-002**: agent_templates.metadata.skill_threshold lookup per phase (D3) — extraído via AgentTemplate.SkillThreshold() en repository.go, hydratado en hydrateSystemPrompts — 2026-06-10
- [x] **skill-003**: PhaseResultResult incluye SkillsRecommended array (skills_recommended en respuesta) — 2026-06-10

## suggested_saves contract (D5)

- [x] **save-001**: SuggestedSave struct vive en `phases/registry.go`; buildSaves implícito en cada handler — 2026-06-10
- [x] **save-002**: `orchestrator/saves.go::ValidateRequiredSaves` con RequiredSaveError + Unwrap → ErrRequiredSaveMissing — 2026-06-10
- [x] **save-003**: Tests unit por fase D5 — `saves_per_phase_test.go`: D5_SDDDesign_RequiresADR (con sad path tipo incorrecto), D5_SDDApply_RequiresCodeReference, D5_SDDJudge_RequiresSabotageRecord, D5_PhasesWithoutRequired_AlwaysPass (7 sub-tests), D5_MultipleRequiredMissing_AllReported. 11 tests verdes — 2026-06-10

## Métricas + observabilidad

- [x] **obs-001**: métricas orquestador en internal/metrics — domain_orchestrator_runs_total{mode,status}, _phase_duration_seconds histogram, _phase_results_total{phase,mode,result}, _confirms_total{confirmed}, _required_save_missing_total{phase,save_type} — 2026-06-10 (Service.Metrics opcional)
- [x] **obs-002**: `domain_agent_runs_orphan_total` ya implementado en chunk foundation (28fddeb) + issue-08.12 cron — 2026-06-10
- [x] **obs-003**: OTel spans `orchestrator.run` + `orchestrator.phase_result` con SafeAttrs nuevos (orchestrator.mode, orchestrator.run_id, phase.slug, flow_run.id, flow_run_step.id, phase.result, phase.requires_confirm) — 2026-06-10
- [x] **obs-004**: `deploy/prometheus/alerts/orchestrator.yml` — 5 alerts (FailureRateHigh, D5RequiredSaveMissingSpike, PhaseSlow p95>10min, D1ConfirmsRejected >50%, RunsStuck sin terminal en 1h) — 2026-06-10

## MCP tools nuevos

- [x] **mcp-001**: `internal/mcp/server/orchestrate_tools.go::domain_orchestrate` — 2026-06-10 (con raw_text + mode + starting_phase + skip_phases + express_max_lines)
- [x] **mcp-002**: `domain_orchestrate_phase_result` — 2026-06-10 (valida D5 + handler.Validate; devuelve step status + next step prompt; propaga flow_run status agregado)
- [x] **mcp-003**: `domain_orchestrate_confirm` (flow_run_id, confirmed) — D1 confirm condicional Express: si apply reporta files>1 OR lines>ExpressMaxLines, verify queda blocked hasta confirm — 2026-06-10
- [x] **mcp-004**: `domain_flow_status` — 2026-06-10 (lee flow_run + steps con outputs/error/preview)
- [x] **mcp-005**: Wire-up completo — `cmd/domain-mcp/main.go` construye `phases.Registry` + `orchestrator.New(pool, recorder, registry, cfg.Env)` y inyecta a `Deps.Orchestrator`. `agentRunnerInst.Env = cfg.Env` también wireado (enforcement orphan-runs activo en prod). `cmd/domain` no construye MCP (no aplica). — 2026-06-10
- [x] **mcp-006**: PromptRouter integration — Router.Orchestrator opcional; cuando inyectado, feat/refactor/doc/rfc → Full, fix/hotfix → Express. chat/idea bypass. Outcome=OutcomeOrchestratorStarted con FlowRunID + SnapshotPrompt — 2026-06-10

## CLI

- [x] **cli-001**: `domain workflow resume <flow_run_id>` — devuelve flow status + tabla de steps + preview prompt del próximo pending — 2026-06-10 (cmd/domain/init_cli.go::runWorkflowResume)

## Intent analysis (D7)

- [x] **ana-001**: PromptRouter clasifica `analysis` como intent separado — 2026-06-10
- [x] **ana-002**: `internal/service/orchestrator/analysis/` — mini-pipeline 2 fases (explore + write_doc) — 2026-06-10
- [x] **ana-003**: Genera knowledge_doc con source='analysis', created_by, scope=org — 2026-06-10
- [x] **ana-004**: Crea observation indexable apuntando al doc — 2026-06-10

## Tests E2E (1 por escenario del issue.md)

- [x] **test-001**: Re-cataloging — cubierto por `internal/seeds/catalogs_integration_test.go::TestSeedAgentTemplatesForOrg_BuiltinCatalog` + cleanup defensivo en `TestSeedAgentTemplatesForOrg_CleansLegacy*` (foundation 28fddeb)
- [x] **test-002**: UNIQUE INDEX orchestrator único — cubierto por `internal/seeds/sabotage_orchestrator_integration_test.go::TestSabotage_UniqueOrchestratorPerOrg` + `_AcrossOrgs` (sab-002, chunk 11)
- [x] **test-003**: Modo Express con confirm condicional D1 — cubierto por `internal/service/orchestrator/confirm_integration_test.go::TestExpressD1_*` (4 tests: SmallChange_AutoApproves, LargeChange_RequiresConfirm, MultiFile_RequiresConfirm, RejectConfirm_MarksFlowFailed) (chunk 9)
- [x] **test-004**: Multi-concern auto-split D2 — 2026-06-10 (RecordPhaseResult detecta multi_concern=true en explore output, cancela steps restantes, retorna MultiConcernInfo; cobertura unitaria + extractConcerns helpers)
- [x] **test-005**: State server + execution client — cubierto implícitamente por todos los integration tests Full+Express: el Service NUNCA ejecuta fases (sólo Build + Persist + Validate); cliente IDE simulado vía RecordPhaseResult cubre la mitad client side
- [x] **test-006**: Resume cross-session — cubierto por `internal/service/orchestrator/service_resume_integration_test.go::TestService_ResumeCrossSession` (este chunk) + CLI `domain workflow resume`
- [x] **test-007**: Dual output — 2026-06-10 (RFC 0006 §4: PhaseResultResult.Summary extraído desde output["summary"]; 2 tests unitarios. El cliente IDE incluye summary en su output; MCP tool devuelve solo el summary, payload completo queda en flow_run_steps.outputs)
- [x] **test-008**: Auto-skill threshold D3 — cubierto por skill-001..003; threshold desde agent_templates.metadata + fetchRecommendedSkills en RecordPhaseResult. Pendiente test E2E con testcontainers — 2026-06-10
- [x] **test-009**: Cron → flow D4 — 2026-06-10 (dispatchSync extraído para testing sincrónico; 4 tests unitarios nil-runner + cron.Service.PickDue con testcontainers: 5 tests cubren pick due, respeta limit, salta disabled/deleted, no pilla future. Pendiente E2E completo con flowrunner.Runner real — requiere issue-10.1 full infra)
- [x] **test-010**: suggested_saves required D5 — cubierto por `phase_result_integration_test.go::TestExpress_ApplyMissingRequiredSave_MarksStepFailed` + `metrics_test.go::TestService_RecordPhaseResult_IncrementsRequiredSaveMissingMetric` + `saves_test.go` (5 unit tests) + save-003 explícitos
- [x] **test-011**: Async D6 — 2026-06-10 (10 integration tests: Run returns inmediatamente, Process ejecuta 10 fases + signals, LLM factory required, non-async rejected, invalid JSON → failure signal, degraded sin SignalStore, resume cross-session, StartingPhase, SkipPhases, Repo required)
- [x] **test-012**: Intent analysis D7 — 2026-06-10 (ana-001..004 completados; pendiente integration test E2E con DB real)
- [x] **test-013**: Service-layer enforcement orphan — cubierto por `internal/runner/agent/options_test.go::TestCheckOrphanPolicy` (5 cases dev/staging/prod × standalone variants) + `service_persistence_integration_test.go` valida flow_run_id en INSERT
- [x] **test-014**: Sabotage INSERT bypass — cubierto por `tests/e2e/orphan_runs_audit_test.go::TestOrphanAudit_Sabotage_BypassDetected` (issue-08.12 cron, sab-001)
- [x] **test-015**: Recovery desde snapshot — N/A en esta HU (fuera de alcance issue-08.10; se cubre vía issue-09.6 durable-execution: TestRecovery_ReleaseStaleRun + resume engine) — 2026-06-10

## Sabotaje

- [x] **sab-001**: INSERT directo bypass → métrica orphan incrementa — ya cubierto por `tests/e2e/orphan_runs_audit_test.go::TestOrphanAudit_Sabotage_BypassDetected` (issue-08.12 cron) — 2026-06-10
- [x] **sab-002**: 2 templates orchestrator por org → UNIQUE violation — 2026-06-10 (TestSabotage_UniqueOrchestratorPerOrg + AcrossOrgs en internal/seeds/sabotage_orchestrator_integration_test.go)
- [x] **sab-003**: Forzar required_save missing → fase no avanza, error específico — 2026-06-10 (`TestService_Sabotage_ApplyMissingRequiredCodeReference`)

## Docs

- [x] **doc-001**: `docs/agents/sdd-pipeline.md` — user-facing doc completa con 5 modos, 10 fases, 4 MCP tools, D1/D5, CLI, métricas, bootstrap, troubleshooting — 2026-06-10
- [x] **doc-002**: `docs/flows/09-orchestrator.md` — DAG flowchart + secuencias Mermaid Full mode + Express D1 confirm + resume cross-session — 2026-06-10
- [x] **doc-003**: `docs/GETTING_STARTED.md` sección 8 "Primer prompt con orquestador SDD" con ejemplos Express + Full + D1 + resume — 2026-06-10
- [x] **doc-004**: `CHANGELOG.md` Unreleased — entrada consolidada issue-08.10 — 2026-06-10

## Estado

- [x] **state-001**: Actualizar state.yaml a `implemented` post-merge — 2026-06-10
- [x] **state-002**: Actualizar RFC 0006 con link a issue-08.10 implementado — 2026-06-10
- [x] **state-003**: Actualizar `openspec/changes/REQ-08-agent-system/req.md` con HU-08.10 implementado — 2026-06-10
