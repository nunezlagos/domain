# Proposal: issue-08.10-sdd-pipeline-orchestrator

## Scope (in)

- Reemplazo del catálogo de 10 `agent_templates` (researcher/coder/...) por 10 alineados a fases SDD (`sdd-orchestrator` + 9 `sdd-<phase>`).
- Cleanup defensivo en `agent_templates_catalog.go` (mismo patrón que `PlansSeeder v2` issue-21.3).
- 1 seeder nuevo: `flow:sdd-pipeline-v1` con `spec` JSONB que encadena las 10 fases vía `flow_run_steps`.
- Service nuevo: `internal/service/orchestrator/Service` (thin, 4 modos: Full/Solo/Detect/Async).
- Enforcement service-layer: `agent_runs` orphan (sin `flow_run_id`) rechazado en prod salvo `WithStandalone(true)`.
- Métrica `domain_agent_runs_orphan_total{org_id, reason}` + alert PrometheusRule.
- CHECK constraint `agent_templates.role IN ('orchestrator', 'phase-worker')` + UNIQUE INDEX parcial `WHERE role='orchestrator'`.
- Modo Async: pausa via `flow_signals` (BIGSERIAL) — diferencial Domain (gentle-ai no tiene).
- 10 tests E2E (1 por escenario) + 1 sabotaje + 1 cron de auditoría orphan.

## Scope (out)

- Wizard adaptive (issue-04.7 v2) — NO se toca. Cuando wizard termina, emite `flow_signal` que el orquestador toma.
- PromptRouter — NO se toca. Sigue clasificando intent y delegando a issuebuilder.
- MCP `domain_prompt` tool — NO cambia la firma; internamente decide si crea un flow_run o sólo wizard.
- Re-implementar parent_run_id (ya existe issue-08.6).
- Re-implementar flow_signals (ya existe en migrations 000060-000063).
- Cambios de schema BD destructivos (no hay drops).

## Cambios

### Schema (1 migration mínima)

```sql
-- migration 000073: agent_templates.role + constraints
ALTER TABLE agent_templates
  ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'phase-worker'
  CHECK (role IN ('orchestrator', 'phase-worker'));

CREATE UNIQUE INDEX IF NOT EXISTS agent_templates_orchestrator_unique_idx
  ON agent_templates (organization_id) WHERE role = 'orchestrator';
```

Backfill: cero rows necesitan ser tocadas (todas defaultean a `phase-worker`, el orchestrator se inserta como nuevo).

### Seeders

- `internal/seeds/agent_templates_catalog.go`: replace 10 entries actuales por 10 nuevos (1 orchestrator + 9 phase-workers).
- `internal/seeds/flows_catalog.go` (NUEVO): seeder de `flow:sdd-pipeline-v1` per-org con spec JSONB.

### Code

- `internal/service/orchestrator/service.go` — Run(mode, prompt) → orchestrator_run_id + flow_run_id
- `internal/service/orchestrator/modes/` — full.go, solo.go, detect.go, async.go
- `internal/service/agent/service.go` — agregar `WithStandalone()` option + check env-aware
- `internal/metrics/` — registrar `domain_agent_runs_orphan_total`

### Tests

- `tests/e2e/orchestrator_test.go` — 10 escenarios
- `internal/service/orchestrator/sabotage_test.go` — orphan bypass
- `cmd/audit-orphan-runs/main.go` (NUEVO) — cron que cuenta orphans

## No-goals

- NO se cambia el wire-up de `cmd/domain/main.go` salvo agregar `orchestratorSvc := &orchestrator.Service{...}` en `runServer`.
- NO se rompe API HTTP existente.

## Stakeholders

- nunezlagos (decisión: Replace+cleanup, híbrido reforzado, 4 modos)
- Domain MCP runtime (consumer del nuevo orchestrator)

## Dependencias

- issue-08.5 agent-templates ✅
- issue-08.6 multi-agent-supervisor ✅
- issue-09.x flows + flow_signals ✅

## Estado

`proposed` — aprobada por usuario en 2026-06-10, lista para implementación.
