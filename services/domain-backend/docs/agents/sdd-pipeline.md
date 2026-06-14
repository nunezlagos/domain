# SDD Pipeline Orchestrator

El orquestador SDD (issue-08.10, RFC 0006) es el patrón canónico de Domain
para convertir un prompt libre del usuario en una secuencia gobernada de
fases ejecutadas por un cliente IDE (Claude Code, Cline, etc.).

**Principio rector:** el servidor mantiene estado + LLM + memoria + skills;
el cliente IDE ejecuta las operaciones reales (`bash`, `edit`, `test`,
`commit`). Esto separa razonamiento del side-effect físico.

---

## Cuándo se activa

Cuando un usuario manda un prompt feature/fix/refactor/doc/rfc/hotfix al
binario `domain-mcp` (o al endpoint HTTP `/api/v1/prompt`), el PromptRouter
clasifica el intent y, si está configurado con `Router.Orchestrator`,
invoca al orquestador en lugar del wizard legacy.

El mapeo intent → modo es:

| intent     | modo orquestador | nota |
|------------|------------------|------|
| `feature`  | **Full** (10 fases) | implementación nueva |
| `refactor` | **Full** | mejora interna |
| `doc`      | **Full** | actualización de docs |
| `rfc`      | **Full** | decisión arquitectónica |
| `fix`      | **Express** (2 fases) | bug fix rápido |
| `hotfix`   | **Express** | urgente |
| `chat`     | bypass | respuesta directa, no SDD |
| `idea`     | bypass | exploración, no SDD |

---

## Los 5 modos

### `express` — fast path 2 fases

`sdd-apply` + `sdd-verify`. Pre-arma ambos prompts up-front. Ideal para
cambios pequeños (≤10 líneas, single-file). Si tras `sdd-apply` el output
declara `files_changed > 1` o `lines_changed > ExpressMaxLines`, dispara
el **confirm condicional D1**: el cliente debe llamar
`domain_orchestrate_confirm` para continuar.

### `full` — pipeline 10 fases

`sdd-explore` → `sdd-spec` → `sdd-propose` → `sdd-design` → `sdd-tasks` →
`sdd-apply` → `sdd-verify` → `sdd-judge` → `sdd-archive` → `sdd-onboard`.

**Lazy build:** sólo el primer step se construye con su prompt. Los demás
quedan con `user_prompt` vacío hasta que la fase anterior reporte
`domain_orchestrate_phase_result`; en ese momento el servidor reconstruye
el prompt usando `PriorOutputs` reales de las fases completadas.

Soporta `starting_phase` (resume desde una fase específica) y `skip_phases`
(omitir slugs concretos).

### `detect` — dry-run

Arma `BuildFullPlan` completo, hidrata los `system_prompt` desde BD, pero
**no persiste** `flow_runs` ni `flow_run_steps`. Devuelve el plan en
memoria para inspección. Si el cliente quiere ejecutar de verdad, vuelve
a invocar con `mode=full`.

### `solo` — server-side LLM

**Pendiente** (`svc-005`). El servidor ejecutaría las fases con un LLM
provider directo (sin cliente IDE colaborador). Requiere inyección de
`llm.Factory` en el `Service`.

### `async` — flow_signals + worker tail

**Pendiente** (`svc-007`). El caller dispara el orquestador y desconecta;
un worker lee `flow_signals` para avanzar el flow en background. Depende
de la infraestructura de issue-09.

---

## Las 10 fases

| slug              | rol                         | D5 required save | retry_policy      |
|-------------------|-----------------------------|------------------|--------------------|
| `sdd-explore`     | analiza prompt + multi-concern | —                | re-emit            |
| `sdd-spec`        | produce issue.md            | —                | re-emit            |
| `sdd-propose`     | proposal.md status=draft    | —                | re-emit            |
| `sdd-design`      | ADRs + test_plan + sabotage_plan | **`adr`**     | re-emit            |
| `sdd-tasks`       | descomposición atómica       | —                | re-emit            |
| `sdd-apply`       | implementación TDD strict    | **`code_reference`** | require-cleanup |
| `sdd-verify`      | valida Gherkin scenarios     | —                | re-emit            |
| `sdd-judge`       | sabotage tests               | **`sabotage_record`** | re-emit         |
| `sdd-archive`     | marca issue archived         | —                | re-emit            |
| `sdd-onboard`     | knowledge_doc opcional       | —                | re-emit            |

Los `system_prompt` de cada handler viven en BD (`agent_templates`
seedeado por `SeedAgentTemplatesForOrg`). El operador del despliegue
puede customizarlos vía UI/MCP sin recompilar el binario.

### D5 contract (RFC 0006)

Las fases marcadas `required=true` exigen que el cliente persista una
`memory_ref` del tipo declarado **antes** de reportar `phase_result`. Si
no aparece en `MemoryRefsSaved`, el servidor:

1. Marca el step `failed` con razón `required_save_missing`
2. Propaga el agregado al `flow_run` → status `failed`
3. Incrementa `domain_orchestrate_required_save_missing_total{phase,save_type}`
4. Devuelve `*RequiredSaveError` envuelto en `ErrRequiredSaveMissing`

**Sticky failure:** el step `failed` no se puede re-marcar. El cliente
debe arrancar un nuevo `flow_run` para reintentar.

### D1 confirm condicional Express

Cuando Express completa `sdd-apply` y el output supera el threshold
(`files_changed > 1` o `lines_changed > ExpressMaxLines`, default 10),
el step `sdd-verify` se marca `blocked` y la respuesta de
`phase_result` incluye `requires_confirm: true`.

El cliente debe llamar `domain_orchestrate_confirm(flow_run_id, confirmed)`:

- `confirmed=true` → step pasa a `pending`, cliente continúa con su prompt
- `confirmed=false` → step pasa a `failed`, flow termina como `failed`

---

## Las 4 MCP tools

### `domain_orchestrate`

Arranca un nuevo `flow_run`. Argumentos:

```json
{
  "raw_text": "implementar export CSV con streaming",
  "mode": "full",
  "starting_phase": "sdd-design",
  "skip_phases": ["sdd-onboard"],
  "express_max_lines": 10
}
```

`raw_text` es obligatorio. Los demás son opcionales (`mode` default `full`).

Devuelve:

```json
{
  "OrchestratorRunID": "...",
  "FlowRunID": "...",
  "Mode": "full",
  "Plan": {
    "Mode": "full",
    "Steps": [{ "ID": "...", "Slug": "sdd-explore", "SystemPrompt": "...", "UserPrompt": "..." }, ...]
  },
  "SnapshotPrompt": "Prompt del primer step para que el cliente arranque"
}
```

### `domain_orchestrate_phase_result`

Reporta el resultado de una fase. Argumentos:

```json
{
  "flow_run_step_id": "...",
  "output": { "summary": "...", "files_changed": [...] },
  "memory_refs_saved": [{ "type": "code_reference", "id": "..." }],
  "duration_ms": 12345
}
```

Devuelve:

```json
{
  "StepID": "...",
  "StepStatus": "completed",
  "FlowRunStatus": "running",
  "NextStepID": "...",
  "NextStepKey": "sdd-verify",
  "NextStepPrompt": "Validá los escenarios Gherkin...",
  "RequiresConfirm": false
}
```

### `domain_orchestrate_confirm`

Resuelve el D1 confirm condicional Express. Argumentos:

```json
{
  "flow_run_id": "...",
  "confirmed": true
}
```

### `domain_flow_status`

Lee el estado completo del `flow_run`. Argumentos:

```json
{ "flow_run_id": "..." }
```

Devuelve el flow + array de steps con status, outputs (truncados) y
preview de `user_prompt`.

---

## CLI complementario

```bash
domain workflow resume <flow_run_id>
```

Muestra una tabla numerada de los steps + el preview del prompt del
próximo step `pending` o `blocked`. Útil para reanudar después de
una sesión cortada.

---

## Métricas Prometheus

| serie                                                            | tipo      |
|------------------------------------------------------------------|-----------|
| `domain_orchestrator_runs_total{mode,status}`                    | counter   |
| `domain_orchestrator_phase_duration_seconds{phase,mode}`         | histogram |
| `domain_orchestrator_phase_results_total{phase,mode,result}`     | counter   |
| `domain_orchestrator_confirms_total{confirmed}`                  | counter   |
| `domain_orchestrator_required_save_missing_total{phase,save_type}` | counter |

---

## Bootstrap requerido por org

Antes de invocar el orquestador, la org debe tener seedeados los dos
catálogos:

```go
seeds.SeedAgentTemplatesForOrg(ctx, pool, orgID) // 11 agent_templates sdd-*
seeds.SeedFlowsForOrg(ctx, pool, orgID)          // flow sdd-pipeline-v1
```

Sin esto, `Service.Run` falla con `ErrFlowNotSeeded` o
`ErrAgentTemplateNotFound`. El boot estándar de `cmd/domain server`
debería correr ambos automáticamente para todas las orgs activas.

---

## Troubleshooting

| síntoma                                       | causa probable                                           | acción                                |
|----------------------------------------------|----------------------------------------------------------|---------------------------------------|
| `ErrFlowNotSeeded`                           | la org no corrió `SeedFlowsForOrg`                       | bootstrap los seeds en boot           |
| `ErrAgentTemplateNotFound: slug=sdd-apply`   | seed v3 de agent_templates ausente                       | `SeedAgentTemplatesForOrg`            |
| `RequiredSaveError: phase X missing saves`   | cliente reportó sin `mem_save` Required del D5           | re-arrancar flow + guardar saves      |
| `ErrFlowRunStepNotPending`                   | reportar phase_result en step ya completed/failed        | usar `domain_flow_status` para verificar |
| `ErrOrgIDRequiredForOrchestrator` en Router  | `Route()` recibió `orgID=nil`                            | propagar `Principal.OrganizationID`   |
| Step `blocked` indefinidamente                | D1 confirm pendiente                                      | llamar `domain_orchestrate_confirm`   |

---

## Lectura adicional

- RFC 0006 `sdd-pipeline-orchestrator` — `docs/rfc/0006-sdd-pipeline-orchestrator.md`
- HU `issue-08.10` — `openspec/changes/REQ-08-agent-system/issue-08.10-sdd-pipeline-orchestrator/`
- Sequence diagram — `docs/flows/09-orchestrator.md`
- Regla SDD strict — `.claude/rules/sdd.md`
