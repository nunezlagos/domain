# HU-08.10-sdd-pipeline-orchestrator

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer que dirige un proyecto SDD con agentes IA
**Quiero** un orquestador thin que delegue a 10 sub-agentes alineados a las fases SDD (`sdd-explore`, `sdd-spec`, `sdd-propose`, `sdd-design`, `sdd-tasks`, `sdd-apply`, `sdd-verify`, `sdd-judge`, `sdd-archive`, `sdd-onboard`), cada uno con contexto aislado
**Para** que el flujo `prompt → HU → impl → verify → archive` sea explícito, reproducible, y no dependa de un agente "todo-en-uno" que pierde contexto a mitad

## Referencia

Patrón inspirado en [`Gentleman-Programming/gentle-ai`](https://github.com/Gentleman-Programming/gentle-ai) (1 orquestador thin + 10 sub-agentes por fase con contexto aislado), extendido con capacidades Domain-native: pause async via `flow_signals`, recovery via `flow_run_step_snapshots`, bidirectional sync via `external_sync_state`.

## Modelo

- **1 orquestador único** (`agent_templates.role = 'orchestrator'`, único por org, `slug = 'sdd-orchestrator'`) — NO ejecuta código, sólo decompone y delega.
- **10 phase-workers** (`agent_templates.role = 'phase-worker'`) con slug `sdd-<phase>`.
- Orquestador invoca workers vía `flow_run_steps` de un flow seed `sdd-pipeline-v1`, con `parent_run_id` propagado.
- **Cada step corre en contexto aislado** — fresh `agent_run` con sólo el `flow_run_step_snapshots.input` filtrado.
- **4 modos**: `Full` (sub-agents nativos del IDE), `Solo` (orquestador inline ES ejecutor), `Detect-only` (dry-run → `proposals.status='draft'` sin apply), `Async` (pausa awaiting human/external signal — diferencial Domain).

## Criterios de aceptación

### Escenario 1: Re-cataloging templates (replace + cleanup defensivo)

```gherkin
Dado que el seeder agent_templates_catalog.go declara 10 templates sdd-*
Y la org dev tiene 10 templates legacy seeded (researcher, coder, ...) con is_user_modified=false
Cuando corre el seeder vNext
Entonces los 10 templates legacy quedan deleted (DELETE FROM agent_templates WHERE
  seed_managed=true AND is_user_modified=false AND slug NOT IN (catalog_actual) AND
  NOT EXISTS (SELECT 1 FROM agent_runs WHERE agent_id IN (... templates_legacy ...) AND status='running'))
Y los 10 sdd-* quedan upserted
Y los templates con is_user_modified=true NO se tocan (preserva customizaciones)
```

### Escenario 2: Role enforcement (único orquestador por org)

```gherkin
Dado que el catálogo declara sdd-orchestrator con role='orchestrator'
Cuando se intenta insertar un segundo template con role='orchestrator' en la misma org
Entonces falla con constraint violation (UNIQUE INDEX donde role='orchestrator')
Y el seeder es idempotente (UPSERT preserva el único existente)
```

### Escenario 3: Orquestador delega a fase con contexto aislado

```gherkin
Dado que sdd-orchestrator recibe input "implementar feature X"
Cuando descompone en sub-tasks
Entonces emite JSON {"subtasks":[{"worker":"sdd-spec","input":{"raw_text":"X","memory_ref":"obs:..."}}, ...]}
Y por cada subtask crea flow_run_steps con step_key="sdd-spec", parent_run_id=orchestrator_run_id
Y el agent_run del worker recibe SÓLO flow_run_steps.input (no el contexto entero del orquestador)
Y flow_run_step_snapshots persiste input + output para replay
```

### Escenario 4: Modo Full (sub-agents nativos del IDE)

```gherkin
Dado que el cliente es Claude Code / OpenCode / Cursor
Cuando el orquestador delega sdd-spec
Entonces emite tool_call al sub-agent nativo del IDE (mode=full)
Y el sub-agent corre en process isolation
Y reporta result al orquestador via tool_result
```

### Escenario 5: Modo Solo (orquestador ES ejecutor)

```gherkin
Dado que el cliente NO soporta sub-agents (curl/script via /api/v1/prompt)
Y el flow.spec declara mode='solo'
Cuando el orquestador delega sdd-spec
Entonces NO spawnea sub-agent; ejecuta el system_prompt de sdd-spec INLINE en su propio agent_run
Y persiste flow_run_step_snapshots igual que mode=full
```

### Escenario 6: Modo Detect-only (dry-run)

```gherkin
Dado que el flow se invoca con mode='detect'
Cuando el orquestador completa las 10 fases
Entonces los outputs persisten en proposals.status='draft' / designs.status='draft' / tasks (no executed)
Y NO se invoca code apply (sdd-apply genera diff pero no escribe filesystem)
Y NO se mutan code_references
```

### Escenario 7: Modo Async (pause + resume via flow_signals)

```gherkin
Dado que el orquestador llega a fase sdd-design
Y la fase requiere approval humano (config: requires_signal=['design_approved'])
Cuando sdd-design termina su output
Entonces flow_runs.status = 'paused_awaiting_human'
Y NO se avanza a sdd-tasks
Cuando llega POST /api/v1/flow-signals con {name:'design_approved', flow_run_id:...}
Entonces flow_signals.delivered_at se setea
Y un worker reanuda el flow desde flow_run_steps siguiente
Y el agent_run del orquestador NO consume tokens durante la pausa
```

### Escenario 8: Service-layer enforcement (orphan runs)

```gherkin
Dado que el cliente intenta crear agent_run sin flow_run_id en prod
Y NO se pasa WithStandalone(true)
Cuando agentRunService.Create() ejecuta
Entonces devuelve ErrOrphanRunNotAllowed
Y la métrica domain_agent_runs_orphan_total{org_id, reason="missing_flow"} no incrementa (porque no se persiste)
Y en dev (DOMAIN_ENV=development) el mismo call sí se permite con log Warn
```

### Escenario 9: Sabotage — bypass del enforcement

```gherkin
Dado que un test intenta INSERT INTO agent_runs (flow_run_id=NULL, ...) en prod via pool directo
Cuando se ejecuta
Entonces el INSERT funciona (no hay CHECK constraint)
Pero el test sabotage espera que un cron de auditoría detecte el orphan dentro de 5min
Y métrica domain_agent_runs_orphan_total se incrementa
Y la alert AlertManager se dispara
```

### Escenario 10: Recovery (mid-fase falla → resume desde snapshot)

```gherkin
Dado que sdd-apply falla en step 3 de 5 (panic, timeout, OOM)
Cuando se reanuda el flow con POST /api/v1/flow-runs/:id/resume
Entonces flow_run_steps con status='failed' se rehydratan desde flow_run_step_snapshots
Y la fase sdd-apply continúa desde el step que falló (no reinicia desde 0)
Y saga_compensation_log registra el restart event
```

## Análisis breve

- **Qué pide realmente:** mapear el patrón gentle-ai (1 orq + 10 phase-workers) sobre las tablas existentes (`agent_templates`, `agents`, `agent_runs`, `flows`, `flow_runs`, `flow_run_steps`, `flow_signals`) + agregar modo Async como mejora Domain-native.
- **Módulos sospechados:** `internal/service/orchestrator` (nuevo), `internal/service/agent`, `internal/service/flow`, `internal/seeds/agent_templates_catalog.go`, `internal/seeds/flows_catalog.go` (nuevo).
- **Riesgos / dependencias:**
  - Depende de HU-08.5 (templates) ✅ implementada
  - Depende de HU-08.6 (supervisor + parent_run_id) ✅ implementada
  - Depende de HU-09.x (flows + flow_signals) ✅ implementada
  - Riesgo: modo Solo puede agotar context window del orquestador si las 10 fases corren inline. Mitigación: budget caps por fase + summary handoff via observations.
  - Riesgo: modo Async pausa indefinida si el signal nunca llega. Mitigación: `flow_runs.timeout_at` con default 7 días.
- **Esfuerzo tentativo:** M (4-6h con tests + sabotaje).
