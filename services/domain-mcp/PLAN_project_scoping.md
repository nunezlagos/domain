# Plan de implementación — Scoping por proyecto (SDD/TDD, skills, reglas) + cableado de `project_id` en el orquestador

> Verificado contra el schema actual (última migración `000159`). **Corrección importante a los hallazgos:** la migración `000142_drop_org_id_columns_all` (destructiva, ya aplicada) **eliminó `organization_id` de TODAS las tablas del schema `public`, todas las FK a `organizations` y las RLS sobre `organization_id`**. Varios hallazgos leyeron las migraciones de *creación* (000050-000111) y reportan `organization_id` que **ya no existe** (`issue_drafts`, `issue_intake_payloads`, `tdd_verifications`). Estado real: el sistema es **org-less**; `project_id` es el eje de tenencia que sobrevive. Esto SIMPLIFICA el plan: no hay que preservar `organization_id` ni reconciliarlo con RLS.

---

## 1. Resumen ejecutivo

La **definición** del workflow SDD (flows, fases, plantillas de agente, y las `platform_policies`) permanece **GLOBAL**: editar un flow o una policy de plataforma afecta a todos los proyectos. Lo que pasa a ser **por-proyecto** es la **ejecución y sus artefactos**: cada `flow_run`, cada documento de la cadena SDD/TDD, cada skill *enlazada* y cada *project_policy* leída en runtime quedan atadas a un `project_id` que entra una sola vez por bootstrap y viaja por toda la cadena `triage → issue → orchestrator → fases → persistencia`.

| Concepto | Global (definición) | Por-proyecto (instancia/scope) |
|---|---|---|
| Workflow SDD (flow, fases, orden) | ✅ `flows`, `flow_phases` | — |
| Plantillas de agente | ✅ `agent_templates` | — |
| Reglas de plataforma | ✅ `platform_policies` | — |
| Reglas de proyecto | — | ✅ `project_policies` (ya tiene `project_id`) |
| Skill (definición) | ✅ una sola tabla `skills` | — |
| Skill **usable** | — | ✅ solo si enlazada vía `project_skills` |
| Documentos SDD/TDD | — | ✅ `project_id` en raíz + tablas issue-facing |
| `flow_runs` (ejecución) | — | ✅ nuevo `project_id` |
| Triage / issues / intake | — | ✅ vía `sdd_requirements.project_id` |

---

## 2. Migraciones DB

Convención de header verificada (de `000107`/`000142`): cada `.up.sql` abre con
```
-- migration: <nombre>
-- author: mnunez@saargo.com
-- issue: <ref>
-- description: <multilínea, indentada con dos espacios>
-- breaking: <true|false>
-- estimated_duration: <estimación>
```
El linter `cmd/db-conventions-lint` corre en pre-commit sobre `.sql` staged. Toda `.up.sql` lleva su `.down.sql`.

> Estrategia transversal para columnas en tablas con datos: **nullable-first**. Agregar `project_id UUID NULL`, backfill en migración separada, y recién en una migración posterior (post-deploy del código) endurecer a `NOT NULL`. **NO** usar `DEFAULT gen_random_uuid()` (el ejemplo del hallazgo SDD es incorrecto: generaría FKs a proyectos inexistentes y violaría la FK). Los índices sobre tablas con tráfico se crean `CONCURRENTLY` (requiere correrlos **fuera de transacción** — ver nota de runner abajo).

### Lote A — Skills N:N (ver §4)

**`000160_create_project_skills.up.sql`** — `breaking: false`, `<1s`
- CREATE TABLE `project_skills(id, project_id FK→projects CASCADE, skill_id FK→skills CASCADE, created_by UUID NULL, created_at)`, `UNIQUE(project_id, skill_id)`.
- Índices: `project_skills_skill_idx(skill_id)`, `project_skills_project_idx(project_id)`.
- *No bloqueante* (tabla nueva, vacía).

**`000161_backfill_project_skills.up.sql`** — `breaking: false`, backfill
- `INSERT INTO project_skills(project_id, skill_id, created_at) SELECT project_id, id, created_at FROM skills WHERE project_id IS NOT NULL AND deleted_at IS NULL ON CONFLICT DO NOTHING;`
- **Decisión de "global"** (ver §10-D): las skills con `project_id IS NULL` NO se enlazan automáticamente. Bajo la nueva regla "no usable si no enlazada", quedarían huérfanas. Recomendación: enlazarlas a TODOS los proyectos existentes en este backfill (opt-in explícito masivo) y eliminar el concepto de "global implícito".

### Lote B — `project_id` en la cadena SDD/TDD (ver §3)

**`000162_add_project_id_sdd_chain.up.sql`** — `breaking: false`, `<1s` (nullable, sin reescritura)
- `ALTER TABLE sdd_requirements   ADD COLUMN project_id UUID NULL REFERENCES projects(id) ON DELETE CASCADE;` (raíz)
- `ALTER TABLE issue_drafts        ADD COLUMN project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;`
- `ALTER TABLE issue_gherkin_scenarios ADD COLUMN project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;`
- `ALTER TABLE issue_tasks         ADD COLUMN project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;`
- `ALTER TABLE issue_code_references ADD COLUMN project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;`
- `ALTER TABLE issue_intake_payloads ADD COLUMN project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;`
- **NO** se agrega a `sdd_proposals`, `sdd_designs`, `issues`, `issue_draft_steps_log`, `tdd_verification_results`, `tdd_sabotage_records` → derivables por FK JOIN (ver §3).

**`000163_create_index_sdd_project_id.up.sql`** — `breaking: false`, índices `CONCURRENTLY`
- `CREATE INDEX CONCURRENTLY sdd_requirements_project_id_idx ON sdd_requirements(project_id);` (+ análogos por tabla).
- ⚠️ **Bloqueante si no es CONCURRENTLY** sobre tablas con datos. CONCURRENTLY no puede ir dentro de una transacción → el runner debe ejecutar este archivo en modo no-transaccional (a confirmar: cómo marca el runner `internal/migrate` una migración sin BEGIN/COMMIT; revisar si soporta una directiva tipo `-- migrate:no-transaction`).

### Lote C — `flow_runs.project_id` (ver §3, orquestador)

**`000164_add_project_id_flow_runs.up.sql`** — `breaking: false`, `<1s`
- `ALTER TABLE flow_runs ADD COLUMN project_id UUID NULL REFERENCES projects(id) ON DELETE CASCADE;`
- `CREATE INDEX CONCURRENTLY flow_runs_project_id_idx ON flow_runs(project_id) WHERE project_id IS NOT NULL;` (separar en `000165` si el runner exige no-transaction por archivo).

### Lote D — Endurecimiento (post-deploy del código, ola siguiente)

**`000166_backfill_and_notnull_project_id.up.sql`** — `breaking: true`, potencialmente pesada
- Backfill de `sdd_requirements.project_id` para filas existentes (ver §10-B: requiere decisión sobre la data legacy sin proyecto).
- Recién acá: `ALTER TABLE sdd_requirements ALTER COLUMN project_id SET NOT NULL;` y `flow_runs` idem **solo si** §10 decide NOT NULL.
- ⚠️ `SET NOT NULL` hace un full-scan con `ACCESS EXCLUSIVE` lock. Mitigación: agregar primero un `CHECK (project_id IS NOT NULL) NOT VALID` + `VALIDATE CONSTRAINT` (no toma lock exclusivo prolongado), y luego el `SET NOT NULL` aprovecha el check ya validado.

### Lote E — Limpieza diferida (ola N+2)

**`0001xx_drop_skills_project_id.up.sql`** — `breaking: true`
- `ALTER TABLE skills DROP COLUMN project_id;` + drop de los partial unique indexes `skills_org_*` (que referencian `organization_id` ya inexistente → **estos índices están rotos hoy**, ver §10-E). Reemplazar por uniques basados en `project_skills` o en `(slug)` según política de unicidad nueva.

---

## 3. Backend Go por servicio (orden de dependencia: schema → repos → services → orchestrator → MCP tools)

**Tablas con `project_id` directo** → su `Create` recibe `projectID`.
**Tablas derivables** → resuelven `project_id` por JOIN, NO cambian firma de inserción.

### 3.1 Schema/structs de fila
- `requirement.Requirement`, `issuebuilder.Draft`, `issue` scenarios, `task.Task`, `traceability.CodeReference`, `intake` payload, `orchestrator.FlowRunInsert` → agregar campo `ProjectID uuid.UUID` (o `*uuid.UUID` mientras sea nullable).

### 3.2 Repos / SQL (INSERT + SELECT)
- `requirement` repo (`service/requirement/service.go:84`): INSERT con `project_id`.
- `issuebuilder` (`service/issuebuilder/service.go:131`): `Start()` persiste `project_id`.
- `issue` (`service/issue/service.go:93,150+`): `insertScenariosTx` agrega `project_id`.
- `task` (`service/task/service.go:99`): `CreateTasks` agrega `project_id`. `CreateVerification`/`CreateSabotage` (`:283,:315`) NO cambian (derivan de `issue_tasks`).
- `traceability` (`service/traceability/service.go`): `AddCodeReference` agrega `project_id`.
- `intake` (`service/intake/service.go:106`): `SubmitInput` agrega `ProjectID`.
- `orchestrator` repo (`service/orchestrator/repository.go:504-559`): `persistPlan` → `FlowRunInsert.ProjectID`; `CreateFlowRun` INSERT con `project_id`.
- `spec` (`service/spec/service.go:89`): `CreateProposal`/`CreateDesign` **sin cambio de firma** — derivan vía `issue_id → issues.req_id → sdd_requirements.project_id`.

### 3.3 Services
- Propagar `projectID` desde el contexto de fase a cada `Create`.

### 3.4 Orchestrator (firmas a tocar, en orden)
1. `OrchestrateInput` (`orchestrator/types.go:80`) → `+ ProjectID uuid.UUID` (nullable con validación, ver §6).
2. `phases.Input` (`phases/registry.go:37`) → `+ ProjectID uuid.UUID`.
3. `Service.Run()` (`orchestrator/service.go:99-284`) → construir `phaseInput` con `ProjectID`.
4. `Service.persistPlan()` (`repository.go:504`) → `ProjectID: in.ProjectID`.
5. `hydrateSystemPrompts()` (`orchestrator/service.go:332`) → inyectar reglas (ver §5). Requiere inyectar `projectpolicy.Repository` en `orchestrator.Service` (`service.go:44`).
6. `fetchRecommendedSkills()` (`orchestrator/skills.go:31`) → pasar `in.ProjectID` a SearchHybrid.
7. `analysis.resolveProjectID()` (`analysis/service.go:153`) → corregir: usar el `project_id` que llega por input; conservar como fallback solo si nil.

### 3.5 MCP tools
- `domain_orchestrate` (`orchestrate_tools.go:30,96`) → `+ project_id` (string opcional); parsear y validar en `handleOrchestrate`.
- `domain_prompt` (`prompt_tools.go:22-74`) → `+ project_id`; pasar a router.
- `promptrouter.RouteWithIntent` (`promptrouter/router.go:168-290`) → `+ projectID *uuid.UUID`, propagar a `OrchestrateInput`.
- Skills tools (`project_skill_tools.go:128`) → query con JOIN `project_skills` (ver §4).

---

## 4. Skills — modelo N:N

**Opción elegida: tabla puente `project_skills` (Opción A).** Fundamento: integridad referencial real (Postgres valida ambos lados), enlazar/desenlazar sin borrar la skill, auditoría por fila (`created_at`/`created_by`), JOIN indexado más barato que `= ANY(array)`, y mapea limpio a un M2M de Django. El array `project_ids[]` se descarta: sin FK, cleanup N+1 al borrar proyecto, y lock-contention en updates concurrentes.

**Migración de las `skills.project_id` existentes:** backfill 1:1 a `project_skills` (`000161`). Luego `skills.project_id` queda **deprecada** (código deja de leerla) y se dropea en ola N+2 (`Lote E`).

**"Global" bajo la regla "solo usable si enlazada":** el concepto de *global implícito* (`project_id IS NULL` visible por fallback) **se elimina**. Una skill solo es usable en los proyectos donde tiene fila en `project_skills`. Lo que hoy era global se materializa como enlace explícito a cada proyecto (decisión en §10-D). Una "skill de plataforma" sigue siendo *una sola definición* compartida; lo que cambia es que su disponibilidad es **opt-in**, no opt-out.

**Resolución (cómo cambia):**
- `project_skill_tools.go:128`: de `WHERE (project_id = $1 OR project_id IS NULL)` → `JOIN project_skills ps ON ps.skill_id = s.id WHERE ps.project_id = $1`.
- `skill.Service.SearchHybrid` (`skill/service.go:408-488`): nueva firma `SearchHybrid(ctx, projectID, query, limit)` (el `orgID` actual es **dead param**: org ya no existe en el schema). Agregar `JOIN project_skills ps ON ps.skill_id = id AND ps.project_id = $projectID` a las CTE BM25 y vector. Esto además corrige el bug reportado de `SearchHybrid` que ignora el scope.
- `GetBySlug`: agregar `projectID` para desambiguar (corrige el bug de "devuelve la primera con ese slug").

**Autoskill (`skill/auto_engine.go`):** `Materialize(ctx, t, projectID)` → dentro de la misma tx: (1) INSERT en `skills` (sin `project_id`, deprecada), (2) `INSERT INTO project_skills(project_id, skill_id) VALUES ($projectID, $newID) ON CONFLICT DO NOTHING`. Así el autoskill **crea + enlaza** solo al proyecto que lo originó. El `projectID` debe llegar desde el `DiscoveredTool`/contexto de la fase que disparó el descubrimiento.

---

## 5. Reglas en el flujo SDD

Hoy `hydrateSystemPrompts` (`orchestrator/service.go:332`) solo rellena `SystemPrompt` desde `agent_templates`. El orquestador **no lee policies** y **no tiene** `projectpolicy.Repository`.

**Inserción del paso "leer reglas":** dentro de `hydrateSystemPrompts`, por cada step:
1. Resolver `projectID` (de `OrchestrateInput`, ya disponible tras §6).
2. Leer reglas con el resolver jerárquico ya existente (`domain_policy_get`/`projectpolicy.Repository`): `platform_policies` (kind `sdd_workflow`, `convention`, `security_rule`, etc.) + `project_policies` del `projectID`. Respetar `override_platform` (true reemplaza; false concatena).
3. **Seleccionar skills enlazadas:** `fetchRecommendedSkills(ctx, projectID, templateSlug)` ahora filtra por `project_skills`.
4. Concatenar al `SystemPrompt`: bloque de reglas (plataforma+proyecto) + bloque de skills recomendadas.

**Exposición al LLM:** el `SystemPrompt` de cada fase pasa a contener, en orden: (a) plantilla de agente, (b) reglas resueltas, (c) skills enlazadas/recomendadas. Las skills disponibles para invocar (tools) siguen filtradas por `project_skills`. Afecta a **todas** las fases (explore→onboard).

---

## 6. Origen del `project_id` en runtime

**Origen único:** `domain_session_bootstrap` (`session_bootstrap_tools.go:87-133`) ya resuelve y **devuelve** `project.id` (match por `project_repositories.url` = git remote, fallback `projects.slug` = basename(cwd)). Ese id es la fuente de verdad.

**Recorrido a cablear:**
1. El cliente recibe `project.id` del bootstrap y lo **pasa explícito** como `project_id` en `domain_prompt` / `domain_orchestrate`.
2. `domain_prompt` → `promptrouter.RouteWithIntent(ctx, rawText, createdBy, projectID, intentOverride)`.
3. Router → `OrchestrateInput.ProjectID`.
4. Orchestrator → `phases.Input.ProjectID` → cada `Create(...)` de la cadena → `flow_runs.project_id`.
5. Triage/intake (`intake.Submit`) recibe `ProjectID` en su input; al *commit* del intake, el `sdd_requirement` creado hereda ese `project_id`, y de ahí desciende a issues/tasks/scenarios.

**Validación:** si `project_id` viene → validar que existe y no está `deleted`. Si viene nil → fallback de `resolveProjectID` (corregido para no tomar "el primero" ciegamente: si hay >1 proyecto activo y no se pasó id → error explícito; si hay exactamente 1 → usarlo con warning). Esto reemplaza el parche `analysis/service.go:153` que ignora el scope.

---

## 7. Admin (Django)

- **Skills** (`maintainers/skills/models.py:39-94`): agregar modelo `ProjectSkill` (M2M `Project`↔`Skill`, `managed=False` sobre `project_skills`). En el admin de Skill: inline/`filter_horizontal` para enlazar/desenlazar proyectos. La skill deja de mostrar `project_id` directo (deprecado).
- **Reglas por proyecto:** hoy `project_policies` solo tiene MCP tools (no REST, no CLI, no admin). Agregar mantenedor Django para CRUD de `project_policies` por proyecto (kind, body_md, `override_platform`, soft-delete) — análogo al de `platform_policies`. Permite agregar/quitar reglas en el tiempo.
- **Issues por proyecto:** una vez `sdd_requirements.project_id` existe, agregar filtro por proyecto en el listado de issues/requirements (JOIN `req → project`).
- **Diagrama SDD** (`config/views.py` `sdd_flow()` + `templates/sdd_flow.html`): ver §8.

---

## 8. Diagrama de secuencia SDD

Archivo: `templates/sdd_flow.html` (estático, alimentado por `views.py:sdd_flow()` vía `pmap`).

**Agregar 1 participante** `ProjectResolver` (x≈480, entre Router y Orchestrator) con su lifeline.

**Agregar 4 mensajes** de setup, tras "Datos→ok" y antes de "explore":
- Router → ProjectResolver: `project_id + rules_filter`
- ProjectResolver → Orchestrator: `project_rules + linked_skills`
- Orchestrator → ProjectResolver: `resolve_skills(project_id)`
- ProjectResolver → Orchestrator: `skill_bindings + context`

**Reflow vertical +78px** en todo lo que está debajo: 10 fases, retornos punteados, fragmento `loop` (y=538→616), loop de reintento (path +78), mensaje final (852→930), nota "modo lite" (+78), lifelines `y2` 864→942, activation bars (Orchestrator height 700→778, Agente 510→588), `viewBox` alto 980→1058.

El `ProjectResolver` representa visualmente la lógica de `hydrateSystemPrompts` (lectura de reglas + selección de skills enlazadas). La estructura de las 10 fases NO cambia (workflow sigue global). Opcional: enriquecer `views.py:sdd_flow()` para inyectar `project_rules`/`linked_skills` si se quiere el diagrama dinámico (no requerido).

---

## 9. Orden de deploy + riesgos + compat hacia atrás

**Orden:**
1. **Migraciones aditivas nullable** (`000160-000165`): tabla puente, `project_id` nullable, índices CONCURRENTLY, backfill `project_skills`. Sin downtime, compatible con código viejo.
2. **Deploy backend Go** que: escribe `project_id` en inserts, lee skills vía `project_skills`, lee reglas en `hydrateSystemPrompts`, cablea `project_id` end-to-end. Sigue tolerando filas legacy con `project_id NULL`.
3. **Deploy admin Django** (M2M skills, mantenedor project_policies, filtros).
4. **Backfill legacy + endurecimiento** (`000166`): asignar `project_id` a data SDD existente (§10-B) y `SET NOT NULL` donde se decida.
5. **Ola N+2:** drop de `skills.project_id` y de los uniques rotos.

**Riesgos / compat:**
- **Data SDD existente sin `project_id`** → bloquea `SET NOT NULL` hasta backfill (§10-B).
- **RLS:** ya NO hay RLS sobre `organization_id` (000142 las dropeó). Si se quiere aislamiento por proyecto a nivel BD habría que crear RLS nuevas sobre `project_id` (hoy el aislamiento es por código). **A confirmar** si se desea RLS por proyecto o se mantiene en código.
- **Uniques de skills rotos:** `skills_org_slug_global_uniq`/`skills_org_project_slug_uniq` referencian `organization_id` inexistente → revisar si siguen vigentes en la BD real (probablemente fueron dropeados por el pre-cleanup de 000142). **A confirmar** y reemplazar.
- **FKs `ON DELETE`:** raíz `sdd_requirements.project_id` → CASCADE (borrar proyecto borra su cadena SDD). Tablas issue-facing satélite → SET NULL para no perder trazabilidad si el patrón lo exige. Revisar coherencia con el CASCADE existente issue→tasks.
- **`tdd_verifications`:** ya tiene `project_id NOT NULL` (sano). Su `organization_id` de la creación 000111 ya fue dropeado; no tocar.
- **CONCURRENTLY fuera de transacción:** confirmar soporte del runner (`internal/migrate`).

---

## 10. DECISIONES ABIERTAS (requieren input del usuario)

**A. ¿`flow_runs.project_id` y `sdd_requirements.project_id` terminan `NOT NULL`?**
Recomendación: **SÍ, NOT NULL tras backfill** (`000166`). Es el eje de tenencia; nullable indefinido reintroduce el parche "primer proyecto". Mantener nullable solo durante la ventana deploy.

**B. ¿Qué hacer con la data SDD/TDD existente sin `project_id`?**
Opciones: (1) asignar todo al único proyecto activo si solo hay uno; (2) crear un proyecto `legacy`/`unassigned` y backfillear ahí; (3) borrar data de prueba.
Recomendación: si hoy es efectivamente single-project, **(1)**. Si hay ambigüedad, **(2)** con proyecto `__legacy__` y marcar para revisión manual. Evitar `NOT NULL` hasta resolver esto.

**C. ¿Se elimina el concepto de "skill global"?**
Recomendación: **eliminarlo como global *implícito*** (no más fallback `project_id IS NULL`). Lo que era global se convierte en enlaces explícitos a cada proyecto vía `project_skills`. Mantiene "una definición compartida" pero cumple "no usable si no enlazada". Confirmar.

**D. Backfill de skills hoy globales (`project_id IS NULL`): ¿enlazar a todos los proyectos o dejar sin enlazar?**
Recomendación: **enlazar a todos los proyectos existentes** en `000161` (preserva comportamiento actual sin romper nada) y a partir de ahí gestionarlas manualmente desde el admin. Alternativa: dejarlas sin enlazar y que un humano las asigne (más limpio, pero deshabilita skills que hoy funcionan). Confirmar tolerancia a romper.

**E. Uniques de skills rotos (referencian `organization_id` inexistente) — ¿nueva regla de unicidad?**
Recomendación: unicidad de `slug` **global por definición** (una tabla, un slug = una skill) ahora que org desapareció y la pertenencia es vía `project_skills`. Confirmar que no se requiere mismo slug con dos definiciones distintas.

**F. ¿RLS por `project_id` a nivel BD, o aislamiento por código?**
Recomendación: **por código** en esta ola (las RLS por org ya se removieron; agregar RLS por proyecto es trabajo grande y ortogonal). Reevaluar en ola futura. Confirmar.

**G. `domain_prompt`/`domain_orchestrate`: ¿`project_id` obligatorio o resoluble del server?**
Recomendación: **opcional en el tool, resuelto del bootstrap del cliente**; si falta y hay ambigüedad → error explícito (no "primer proyecto"). Confirmar.
