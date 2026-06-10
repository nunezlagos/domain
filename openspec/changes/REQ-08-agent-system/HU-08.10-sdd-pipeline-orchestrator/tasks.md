# Tasks: HU-08.10-sdd-pipeline-orchestrator

## Schema (1 migration)

- [ ] **mig-001**: `000073_agent_templates_role.up.sql` — ADD COLUMN role + CHECK + UNIQUE INDEX parcial (where role='orchestrator')
- [ ] **mig-002**: `000073_agent_templates_role.down.sql` — DROP INDEX + DROP CONSTRAINT + DROP COLUMN

## Seeders

- [ ] **seed-001**: Replace 10 entries en `internal/seeds/agent_templates_catalog.go`:
  - 1 × sdd-orchestrator (role='orchestrator', HandoffPolicy='forbid')
  - 9 × sdd-explore / sdd-spec / sdd-propose / sdd-design / sdd-tasks / sdd-apply / sdd-verify / sdd-judge / sdd-archive / sdd-onboard
- [ ] **seed-002**: Agregar cleanup defensivo en el seeder (DELETE WHERE seed_managed=true AND is_user_modified=false AND slug NOT IN (catálogo) AND no_active_runs)
- [ ] **seed-003**: NUEVO `internal/seeds/flows_catalog.go` con `flow:sdd-pipeline-v1`:
  - spec JSONB con 10 steps secuenciales
  - cada step asigna step_key=sdd-<phase>, agent_template_slug=sdd-<phase>
  - per-org via SeedFlowsForOrg

## Service (`internal/service/orchestrator/`)

- [ ] **svc-001**: `service.go` — Service struct + Run(ctx, OrchestrateInput) (orchestrator_run_id, flow_run_id, error)
- [ ] **svc-002**: `OrchestrateInput` struct con Mode, RawText, StartingPhase, SkipPhases, AsyncTimeout
- [ ] **svc-003**: `modes/full.go` — delega via parent_run_id chain
- [ ] **svc-004**: `modes/solo.go` — inline execution, mismo agent_run
- [ ] **svc-005**: `modes/detect.go` — dry_run=true en flow.spec, persiste a status='draft'
- [ ] **svc-006**: `modes/async.go` — emite flow_signals; reanuda con worker que tail flow_signals.delivered_at IS NULL
- [ ] **svc-007**: `phases/` — un archivo por fase con su system_prompt + input_schema + output_schema

## Service enforcement

- [ ] **enf-001**: `internal/service/agent/option.go` — agregar `Option` pattern con `WithStandalone()`
- [ ] **enf-002**: `internal/service/agent/service.go::Create` — si flow_run_id nil AND !standalone AND env=='production' → ErrOrphanRunNotAllowed
- [ ] **enf-003**: `internal/service/agent/errors.go` — definir ErrOrphanRunNotAllowed con code y docs_url

## Métricas + observabilidad

- [ ] **obs-001**: `internal/metrics/orchestrator.go` — registrar phase_duration_seconds histogram + runs_total counter
- [ ] **obs-002**: `internal/metrics/agent.go` — registrar agent_runs_orphan_total counter (labels: org_id, reason)
- [ ] **obs-003**: OTel span por fase con flow_run_step.id como attribute
- [ ] **obs-004**: `deploy/prometheus/alerts/orchestrator.yml` — alert orphan_runs > 0 por 5min

## MCP tool

- [ ] **mcp-001**: `internal/mcp/tools/orchestrate.go` — implementar domain_orchestrate tool
- [ ] **mcp-002**: Wire-up en `cmd/domain-mcp/main.go` y `cmd/domain/main.go::runServer`
- [ ] **mcp-003**: Integrar con PromptRouter — si intent es feature/fix/refactor/hotfix/rfc/doc, router invoca orchestratorSvc.Run() con mode='full' default

## Cron de auditoría

- [ ] **cron-001**: `cmd/audit-orphan-runs/main.go` — cuenta agent_runs con flow_run_id IS NULL desde último ack
- [ ] **cron-002**: Wire-up en flujo cron interno (REQ-26 cron-runner ya existe)

## Tests E2E

- [ ] **test-001**: Re-cataloging — seeder borra legacy + inserta sdd-* (escenario 1 del hu.md)
- [ ] **test-002**: UNIQUE INDEX orchestrator único por org (escenario 2)
- [ ] **test-003**: Delegación con contexto aislado — sub-agent solo recibe input (escenario 3)
- [ ] **test-004**: Modo Full — parent_run_id chain (escenario 4)
- [ ] **test-005**: Modo Solo — inline execution (escenario 5)
- [ ] **test-006**: Modo Detect — proposals queda status='draft' (escenario 6)
- [ ] **test-007**: Modo Async — pause + signal + resume (escenario 7)
- [ ] **test-008**: Orphan run rechazado en prod sin flag (escenario 8)
- [ ] **test-009**: Recovery desde snapshot (escenario 10)
- [ ] **test-010**: PromptRouter → orchestrator integration

## Sabotaje (mandatory por sdd.md)

- [ ] **sab-001**: INSERT directo en BD bypaseando service → métrica orphan se incrementa dentro 5min
- [ ] **sab-002**: Loop orchestrator → orchestrator (intentar hijo con role='orchestrator') → CHECK depth ≤ 1 lo rechaza

## Docs

- [ ] **doc-001**: `docs/agents/sdd-pipeline.md` — descripción del flow + 4 modos con ejemplos
- [ ] **doc-002**: `docs/flows/09-orchestrator.md` — diagrama Mermaid del nuevo flow (sumar al set existente)
- [ ] **doc-003**: Actualizar `docs/GETTING_STARTED.md` con sección "Primer prompt con orquestador"
- [ ] **doc-004**: Actualizar `CHANGELOG.md` Unreleased con HU-08.10

## Estado

- [ ] **state-001**: Actualizar state.yaml a `implemented` post-merge
- [ ] **state-002**: Cerrar entrada en REQ-08 req.md si aplica
