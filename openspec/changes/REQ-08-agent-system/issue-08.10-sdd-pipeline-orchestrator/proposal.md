# Proposal: issue-08.10-sdd-pipeline-orchestrator

## Scope (in)

- Reemplazo del catálogo de 10 `agent_templates` por 1 orquestador + 9 phase-workers (slugs `sdd-*`)
- Cleanup defensivo en `agent_templates_catalog.go` (patrón `PlansSeeder v2`)
- Seeder nuevo: `flow:sdd-pipeline-v1` con `spec` JSONB de 10 fases
- Service nuevo: `internal/service/orchestrator/` con 5 modos (Express/Full/Solo/Detect/Async)
- Enforcement service-layer: `agent_runs` orphan rechazado en prod salvo `WithStandalone(true)`
- Métrica `domain_agent_runs_orphan_total{org_id, reason}` (incrementada por cron de issue-08.12)
- Métrica `domain_orchestrator_phase_duration_seconds{phase, mode}`
- CHECK + UNIQUE INDEX parcial `agent_templates.role`
- Modo Async via `flow_signals` (diferencial Domain vs gentle-ai)
- Auto-skill inyección por fase (consume issue-05.4 implementada)
- MCP tools nuevos: `domain_orchestrate`, `domain_orchestrate_phase_result`, `domain_orchestrate_confirm`, `domain_flow_status`
- CLI nuevo: `./bin/domain workflow resume <flow_run_id>`
- Intent `analysis` (mini-pipeline 2 fases que genera `knowledge_doc`)
- 15 tests E2E (1 por escenario) + 1 sabotaje + tests integration por modo

## Scope (out)

- Wizard adaptive (issue-04.7 v2) — NO se toca. sdd-spec lo invoca como sub-rutina.
- PromptRouter — NO se cambia firma. Internamente decide si crea orchestrator_run o solo wizard.
- Schema BD destructivo — sólo 1 migration aditiva (`agent_templates.role` + CHECK + INDEX)
- Re-implementar parent_run_id (ya existe issue-08.6)
- Re-implementar flow_signals (ya existe REQ-09)
- Re-implementar circuit breaker (es issue-12.6, bloqueante separado)
- Re-implementar heartbeat watcher (es issue-08.11, bloqueante separado)
- Re-implementar orphan audit cron (es issue-08.12, bloqueante separado)
- Web UI para visualizar flows (issue futura)

## Cambios

### Schema (1 migration aditiva)

```sql
-- 000074_agent_templates_role.up.sql
ALTER TABLE agent_templates
  ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'phase-worker'
  CHECK (role IN ('orchestrator', 'phase-worker'));

CREATE UNIQUE INDEX IF NOT EXISTS agent_templates_orchestrator_unique_idx
  ON agent_templates (organization_id) WHERE role = 'orchestrator';
```

### Seeders

- `internal/seeds/agent_templates_catalog.go` v3: replace 10 entries por 1 orchestrator + 9 phase-workers
- `internal/seeds/flows_catalog.go` (NUEVO): seeder de `flow:sdd-pipeline-v1` per-org con spec JSONB DAG

### Code Go

- `internal/service/orchestrator/service.go` — Run(ctx, OrchestrateInput) (orchestrator_run_id, flow_run_id, error)
- `internal/service/orchestrator/modes/{full,solo,detect,async,express}.go` — 1 file por modo
- `internal/service/orchestrator/phases/sdd_*.go` — 10 files con system_prompt + input_schema + output_schema por fase
- `internal/service/agent/option.go` — Option pattern con WithStandalone()
- `internal/service/agent/errors.go` — ErrOrphanRunNotAllowed
- `internal/metrics/orchestrator.go` — registrar histograms + counters
- `internal/mcp/tools/orchestrate.go` — MCP tools
- `cmd/domain/workflow_resume.go` — CLI subcomando

### Tests

- `tests/e2e/orchestrator_test.go` — 15 escenarios
- `internal/service/orchestrator/sabotage_test.go` — orphan bypass
- `internal/service/orchestrator/modes/*_test.go` — uno por modo

## Stakeholders

- nunezlagos (decisiones D1-D7 RFC 0006)

## Dependencias

| Issue | Estado | Por qué bloquea |
|---|---|---|
| issue-12.6 mcp-tool-resilience | implementación parcial (CB+LRU pendientes) | Sin CB, MCPs externos pueden colgar al orquestador |
| issue-08.11 heartbeat-watcher-cron | NO existe | Sin esto los flow_run_steps stuck quedan zombis |
| issue-08.12 orphan-runs-audit-cron | NO existe | Sin esto el enforcement híbrido no tiene auditoría |
| issue-05.4 auto-skill-engine | implementada | Consumida por el orquestador en cada fase |
| issue-08.5 agent-templates | implementada | Modelo de templates ya existe |
| issue-08.6 multi-agent-supervisor | implementada | parent_run_id + budget hierarchy ya existen |
| REQ-09 flows + flow_signals | implementado | Estado machine + pause/resume |
| REQ-10 crons + scheduler | implementado | Disparo de flows con leader election |
| issue-04.7 wizard-adaptive v2 | implementada | sdd-spec lo invoca como sub-rutina |

## Estado

`proposed` — bloqueado por 12.6/08.11/08.12. Implementación en orden 12.6 → 08.11 → 08.12 → 08.10.
