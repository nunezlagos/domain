# Design: issue-08.10-sdd-pipeline-orchestrator

## Contexto

Domain hoy tiene 10 `agent_templates` con slugs rol-genérico (`researcher`, `coder`, `tester`, etc.) + 1 `supervisor` marcado con `metadata.pattern = 'multi-agent-orch issue-08.6'`. El catálogo es **flat**: no hay distinción visual entre orquestador y workers. El patrón gentle-ai (Gentleman-Programming) demuestra que **alineación slug↔fase-SDD** da mejor reproducibilidad porque cada fase es un sub-agente fresh.

Domain ya tiene infraestructura más rica que gentle-ai:
- `flow_signals` (BIGSERIAL append) para async
- `flow_run_step_snapshots` para deterministic replay
- `entity_state_transitions` para audit append-only
- `external_sync_state` para bidirectional Jira/GitHub
- `observations` + `knowledge_chunks` para memoria con embedding (gentle-ai usa Engram externamente)

Por lo tanto: **adopto el patrón slug↔fase de gentle-ai + agrego modo Async como mejora Domain-native**.

## Decisiones arquitectónicas

### ADR-1: Re-cataloging vs Soft-delete

**Decisión:** Re-cataloging con cleanup defensivo en el seeder (mismo patrón que `PlansSeeder v2` issue-21.3).

**Alternativas consideradas:**
- A. Soft-delete vía `deleted_at NOT NULL` en agent_templates legacy.
- B. ✅ **Sacar del catálogo + cleanup defensivo en el seeder**.
- C. Mantener viejos + agregar nuevos en paralelo (20 templates total).

**Por qué B:** Domain es local-only sin prod. El "history preservado" del soft-delete es teatro. El cleanup defensivo en seeder es el patrón establecido y respeta `is_user_modified=true`. No introduce data zombies. Aprobado por usuario.

**Tradeoff aceptado:** orgs con agent_runs activos sobre templates legacy fallarán el cleanup por la cláusula `NOT EXISTS (SELECT 1 FROM agent_runs WHERE status='running')`. Tienen que esperar a que terminen. Es comportamiento correcto.

### ADR-2: Enforcement orphan agent_runs

**Decisión:** Híbrido reforzado.

**Alternativas:**
- A. Hard: `agent_runs.flow_run_id NOT NULL` + CHECK constraint.
- B. Soft: warning en service layer.
- C. ✅ **Híbrido:** service layer rechaza en prod salvo `WithStandalone(true)` + métrica `domain_agent_runs_orphan_total` + cron de auditoría + alert.

**Por qué C:**
- Hard (A) rompe debugging y scripts ad-hoc (alguien troubleshooting en staging necesita un agent_run aislado).
- Soft (B) sólo no es suficiente — alguien puede INSERT directo en BD bypaseando el service.
- Híbrido (C) da:
  - Bloqueo en prod por default (catch del 99% de casos)
  - Escape válido para troubleshooting (`WithStandalone()` explícito)
  - Visibility para los casos que bypaseen el service (métrica + cron + alert)
  - Test sabotaje que demuestra que la métrica funciona

**Tradeoff aceptado:** un INSERT directo en BD no es bloqueado a nivel schema, pero queda visible vía métrica dentro de 5min. Trade-off correcto: privilege separation + Datadog/Prometheus visibility en lugar de constraint dura.

### ADR-3: 4 modos (Full / Solo / Detect / Async)

**Decisión:** soportar los 3 modos de gentle-ai + agregar Async como diferencial.

| Modo | Cuándo | Mecanismo |
|---|---|---|
| **Full** | Claude Code / OpenCode / Cursor (default) | `agent_runs` con `parent_run_id` + sub-agent nativo via MCP tool |
| **Solo** | API/MCP sin sub-agent support | `flow_run_steps` ejecutados secuencialmente en el mismo proceso del orquestador |
| **Detect-only** | CI/CD dry-run | `flows.spec.dry_run=true` → outputs persisten en `proposals.status='draft'` sin apply |
| **Async** ⭐ | Pause/resume cross-session | `flow_runs.status='paused_awaiting_*'` + `flow_signals` (BIGSERIAL) |

**Modo Async — por qué Domain lo necesita pero gentle-ai no:**
- gentle-ai corre en un IDE con humano presente. Async no aplica.
- Domain corre como server HTTP/MCP + tiene `flow_signals` heredado de REQ-09.
- Use cases concretos:
  - sdd-design pausa hasta que stakeholder apruebe en Slack (issue-20.x)
  - sdd-tasks pausa hasta que PR de upstream se mergee
  - sdd-apply pausa esperando `domain workflow approve <flow_run_id>`

**Riesgo Async:** pausa indefinida. Mitigación: `flow_runs.timeout_at` con default 7d, configurable via `flow.spec.async_timeout`.

### ADR-4: Mapeo fase ↔ tabla SDD

```
sdd-explore   → intake_payloads (source=agent_orchestrator) + memoria (observations search)
sdd-spec      → issue_drafts (delega al wizard adaptive issue-04.7)
sdd-propose   → proposals (status: draft → approved)
sdd-design    → designs (arch_decisions + tdd_plan + alternatives)
sdd-tasks     → tasks (descompone implementación)
sdd-apply     → code_references + commit
sdd-verify    → verification_results
sdd-judge     → sabotage_records (TDD strict step 4)
sdd-archive   → entity_state_transitions (to_state='archived')
sdd-onboard   → knowledge_docs + platform_policies (genera docs para nuevos devs)
```

**Cero schema nuevo.** Todas las tablas existen. Sólo se agrega `agent_templates.role`.

### ADR-5: Orquestador como tool MCP

El orquestador se expone como nuevo MCP tool `domain_orchestrate`:

```jsonc
{
  "name": "domain_orchestrate",
  "description": "Ejecuta pipeline SDD completo desde un prompt natural",
  "inputSchema": {
    "raw_text": "string",
    "mode": "full | solo | detect | async",
    "starting_phase": "sdd-explore | ...",  // optional, default sdd-explore
    "skip_phases": ["sdd-onboard"]          // optional
  }
}
```

El `PromptRouter` existente queda como gate de entrada — si intent es feature/fix/refactor, el router invoca `orchestratorSvc.Run()`. Si es chat/idea, replica directo (sin entrar al pipeline).

## Patrones aplicados

- **Strategy** — 4 modos como `orchestrator.Mode` interface
- **Saga** — `saga_compensation_log` ya existe para rollback per-phase
- **State machine** — `flow_runs.status` ya tiene paused_awaiting_*
- **Repository** — `agent_templates` accedido sólo via service
- **Adapter** — MCP tool `domain_orchestrate` adapta a `orchestratorSvc.Run()`

## Alternatives rechazadas

| Alt | Por qué no |
|---|---|
| Crear nuevo schema de "phases" | Redundante con `flow_run_steps.step_key` |
| Hard CHECK orphan agent_runs | Rompe debugging legítimo |
| Modo único Full | Pierde valor de async + dry-run |
| Re-implementar handoff | Ya existe issue-08.6/08.7 |

## Riesgos identificados

| Riesgo | Probabilidad | Mitigación |
|---|---|---|
| Modo Solo agota context window con 10 fases inline | Media | Budget caps por fase + summary handoff via observations |
| Async pause indefinida | Media | `flow_runs.timeout_at` default 7d + alert si pausa > 3d |
| Loop orchestrator → orchestrator | Baja (CHECK depth) | `flow_run_steps.depth ≤ 1` (orq no puede invocarse a sí mismo) |
| Orphan cron false positives en dev | Alta | Cron sólo corre con `DOMAIN_ENV=production` |
| Templates legacy quedan en BD por agent_runs activos | Media | Aceptado — cleanup en próximo seed run |

## Observabilidad

- **Métrica:** `domain_orchestrator_phase_duration_seconds{phase, mode}` histogram
- **Métrica:** `domain_orchestrator_runs_total{mode, status}` counter
- **Métrica:** `domain_agent_runs_orphan_total{org_id, reason}` counter (enforcement)
- **Trace:** OTel span por fase con `flow_run_step.id` como span_id
- **Log:** info en cada `flow_run_steps.status` transition
