# issue-08.10-sdd-pipeline-orchestrator

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** alta
**Tipo:** feature
**RFC:** `docs/rfc/0006-sdd-pipeline-orchestrator.md` (accepted 2026-06-10)

## Historia / Tarea

**Como** developer dirigiendo un proyecto SDD en Domain
**Quiero** un orquestador thin que delegue a 10 phase-workers (`sdd-orchestrator` + 9 `sdd-<phase>`) con contexto aislado por fase, manteniendo el modelo donde el cliente IDE ejecuta todo el trabajo real
**Para** que el flujo `prompt → issue → impl → verify → archive` sea explícito, reproducible, resumable cross-session, y consuma piezas existentes (`auto-skill-engine`, `crons`, `flow_signals`) en lugar de re-inventar

## Referencia

Patrón inspirado en [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai), extendido con capacidades Domain-native. Decisiones D1-D7 en RFC 0006.

## Modelo

- **1 orquestador único** (`agent_templates.role='orchestrator'`, único por org, slug `sdd-orchestrator`) — descompone + decide siguiente fase, NO ejecuta workspace
- **9 phase-workers** (`agent_templates.role='phase-worker'`) con slugs `sdd-{explore,spec,propose,design,tasks,apply,verify,judge,archive,onboard}`
- **Modelo de ejecución:** server-side state + LLM + memoria + skills; **cliente IDE ejecuta** bash/edit/test/commit/grep
- **5 modos:** Express (sub-S sin pipeline completo), Full (default), Solo (inline server-side LLM), Detect-only (dry-run), Async (pause via `flow_signals`)

## Criterios de aceptación

### Escenario 1: Re-cataloging templates (replace + cleanup en seeder)

```gherkin
Dado que el seeder agent_templates_catalog.go declara 10 templates sdd-*
Y la org dev tiene 10 templates legacy (researcher, coder, ...) con is_user_modified=false
Cuando corre el seeder vNext
Entonces los 10 legacy quedan deleted (WHERE seed_managed=true AND is_user_modified=false
  AND slug NOT IN (catalog_actual) AND NOT EXISTS agent_runs en running)
Y los 10 sdd-* quedan upserted
Y los templates con is_user_modified=true NO se tocan
```

### Escenario 2: Role enforcement (orquestador único por org)

```gherkin
Dado que el catálogo declara sdd-orchestrator con role='orchestrator'
Cuando se intenta insertar un segundo template con role='orchestrator' en la misma org
Entonces falla con UNIQUE INDEX violation (parcial WHERE role='orchestrator')
Y el seeder es idempotente (UPSERT preserva el único existente)
```

### Escenario 3: D1 — Modo Express con confirm condicional

```gherkin
Dado que el classifier devuelve scope='single-file' y diff_estimated_lines=5
Cuando el orquestador arranca
Entonces ejecuta sdd-apply + sdd-verify directo (2 fases, no las 10)
Y NO pregunta al user antes del commit (diff ≤ 10 líneas)

Dado que classifier devuelve scope='single-file' pero diff_estimated_lines=25
Cuando el orquestador arranca
Entonces pausa antes de sdd-apply commit y muestra diff completo + espera OK explícito
Y solo procede al commit tras tool_call domain_orchestrate_confirm
```

### Escenario 4: D2 — Multi-concern auto-split condicional

```gherkin
Dado que sdd-explore detecta 2 concerns separables (RBAC en X, rate-limit en Y)
Y AMBOS son scope='single-file'
Cuando el orquestador procesa
Entonces auto-divide en 2 flow_runs paralelos sin preguntar
Y cada uno corre su pipeline Express independiente

Dado que sdd-explore detecta 2 concerns Y al menos UNO es scope='multi-file'
Cuando el orquestador procesa
Entonces pausa con flow_signals.name='multi_concern_decision'
Y propone interactivamente (a) split (b) merged (c) sólo #1
Y avanza tras recibir signal con la decisión
```

### Escenario 5: Estado server + ejecución cliente IDE

```gherkin
Dado que el orquestador delega sdd-explore
Cuando el cliente IDE recibe la response del MCP tool domain_orchestrate
Entonces el response incluye {phase, prompt, tools_available, skills_recommended, suggested_saves}
Y NO contiene comandos shell ejecutados por el server
Y el cliente IDE ejecuta grep/read sobre su workspace real
Y reporta resultado via domain_orchestrate_phase_result(step_id, output)
Y el server persiste en flow_run_step_snapshots + avanza state machine
```

### Escenario 6: Resume cross-session

```gherkin
Dado que un flow_run quedó status='paused_awaiting_human' por sdd-design
Y el cliente IDE perdió contexto (compaction o session close)
Cuando una nueva conversación arranca y llama domain_flow_status
Entonces devuelve {flow_run_id, current_phase: sdd-design, awaiting: human, since: <timestamp>}
Cuando el user ejecuta domain workflow resume <flow_run_id>
Entonces el cliente IDE recibe el último flow_run_step_snapshot.output + el prompt formulado por sdd-design
Y puede continuar como si nunca se hubiera pausado
```

### Escenario 7: Dual output (verbose BD + summary IDE)

```gherkin
Dado que cada fase emite output
Cuando termina sdd-design
Entonces flow_run_step_snapshots.output contiene el payload verbose completo (ADRs, intermediate thoughts)
Y el response MCP al cliente IDE contiene ÚNICAMENTE summary 1-line:
  "✓ sdd-design generó 3 ADRs (153 líneas), próximo: sdd-tasks"
Y NO se filtra info sensible al chat
```

### Escenario 8: D3 — Auto-skill inyectado en cada fase

```gherkin
Dado que el orquestador formula prompt para sdd-apply
Y POST /api/skills/recommend devuelve 3 skills con relevance > 0.7 (threshold de sdd-apply en metadata)
Cuando el response al cliente IDE se construye
Entonces incluye skills_recommended con [{slug, relevance, description}] de los 3 skills
Y el cliente IDE puede invocarlos sin búsqueda adicional
```

### Escenario 9: D4 — Cron disparando flow del orquestador

```gherkin
Dado que existe un cron user-defined con target_type='flow', target_id=<sdd-pipeline-v1 uuid>
Y inputs={raw_text:"audita seguridad de los handlers tocados esta semana", mode:"detect"}
Cuando el scheduler lo dispara según cron_expression
Entonces NO pasa por PromptRouter (no hay clasificación needed; inputs ya declaran mode)
Y crea agent_runs sdd-orchestrator con flow_run_id apuntando a flow nuevo
Y NO consume API key del user en LLM classifier (eficiencia)
```

### Escenario 10: D5 — suggested_saves required en críticos

```gherkin
Dado que sdd-design termina y emite 3 ADRs
Cuando responde al cliente IDE
Entonces suggested_saves incluye 3 entries con required=true (type='decision', topic, content_hint)
Cuando el cliente IDE reporta phase_result sin ejecutar los domain_mem_save correspondientes
Entonces el orquestador NO avanza a sdd-tasks
Y devuelve error code='required_save_missing' con la lista de topics faltantes
Y el cliente IDE debe ejecutar los save y re-reportar
```

### Escenario 11: D6 — Modo Async sólo en Full y Detect

```gherkin
Dado que el classifier devuelve scope='multi-file' y se invoca con mode='full'
Y el flow tiene config requires_signal=['design_approved'] en sdd-design
Cuando sdd-design termina
Entonces flow_runs.status = 'paused_awaiting_human'
Y se persiste el snapshot completo

Dado que un cliente intenta invocar mode='express' Y async=true
Cuando el orquestador valida el input
Entonces devuelve error code='async_mode_unsupported' con message="Async sólo disponible en Full y Detect"
```

### Escenario 12: D7 — Intent analysis genera knowledge_doc

```gherkin
Dado que el PromptRouter clasifica intent='analysis'
Cuando ejecuta el mini-pipeline (analysis-explore + analysis-write-doc)
Entonces crea un knowledge_doc con source='analysis', created_by=<user_id>, visible a toda la org
Y crea una observation indexable apuntando al doc
Y RBAC normal aplica (member puede ver, admin puede borrar)
Y la response al cliente IDE es el doc renderizado
Y NO crea issue ni proposal ni design (no es SDD)
```

### Escenario 13: Service-layer enforcement orphan runs

```gherkin
Dado que un test intenta agentRunService.Create(flow_run_id=nil) en prod sin WithStandalone(true)
Cuando ejecuta
Entonces devuelve ErrOrphanRunNotAllowed
Y la métrica domain_agent_runs_orphan_total NO se incrementa (no se persistió)
Y el log incluye razón="missing_flow_run_id"

Dado que el mismo call ejecuta con WithStandalone(true)
Entonces crea agent_runs con flow_run_id=NULL
Y NO devuelve error
Y la métrica domain_agent_runs_standalone_total{reason="debug"} se incrementa
```

### Escenario 14: Sabotage — bypass via INSERT directo

```gherkin
Dado que un test ejecuta INSERT INTO agent_runs (flow_run_id=NULL, ...) via pool directo
Cuando se inserta (no hay CHECK constraint que lo impida)
Entonces dentro de 5min el cron de issue-08.12 detecta el orphan
Y la métrica domain_agent_runs_orphan_total{reason='bypass_service_layer'} se incrementa
Y AlertManager dispara alerta orphan_runs > 0 por 5min
```

### Escenario 15: Recovery desde snapshot

```gherkin
Dado que sdd-apply falla mid-fase (panic, timeout, OOM)
Y flow_run_steps.status='failed' con last_heartbeat_at 7min atrás
Cuando se ejecuta POST /api/v1/flow-runs/:id/resume
Entonces lee flow_run_step_snapshots.output del último step completed
Y rehidrata el contexto al inicio de sdd-apply
Y según retry_policy del template (require-cleanup para sdd-apply) corre primero saga_compensation_log para rollback parcial
Y luego re-ejecuta la fase desde 0 con contexto rehydratado
```

## Análisis breve

- **Qué pide:** mapear patrón orquestador-fase sobre tablas existentes + agregar 4 modos (Express + Async son los nuevos vs gentle-ai) + integration con auto-skill (issue-05.4) + crons (REQ-10) + flow_signals (REQ-09)
- **Módulos:** `internal/service/orchestrator` (nuevo), `internal/service/agent` (extender WithStandalone), `internal/seeds/agent_templates_catalog.go` (replace), `internal/seeds/flows_catalog.go` (nuevo), `internal/mcp/tools/orchestrate.go` (nuevo MCP tool)
- **Dependencies bloqueantes:** issue-12.6 (CB+LRU pendientes), issue-08.11 (heartbeat-watcher), issue-08.12 (orphan-audit) — sin las 3 NO es prod-ready
- **Esfuerzo:** L (8-10h con tests + sabotaje + integración)
- **Riesgos:** modo Solo puede agotar context (mitigación: budget caps), Async pausa indefinida (mitigación: timeout_at default 7d)
