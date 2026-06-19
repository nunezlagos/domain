# issue-42.1-taxonomia-y-catalogo

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** alta
**Tipo:** feature (source of truth / fundación)

## Historia de usuario

**Como** arquitecto del schema y operador del admin `/database`
**Quiero** una taxonomía oficial que asigne a cada una de las 97 tablas un prefijo de funcionalidad, un grupo y un label legible, materializada en una tabla `table_catalog` consultable
**Para** poder agrupar, ordenar y etiquetar las tablas en el explorador de base de datos y tener una ÚNICA fuente de verdad que gobierne los renames posteriores (HUs 42.2+), sin renombrar ni borrar nada todavía

## Alcance explícito

Esta HU **NO** renombra ni dropea ninguna tabla. SOLO:

1. Fija la convención de naming (`## Convención`).
2. Documenta el mapa completo `prefijo → grupo → label` de las 97 tablas (`## Mapa de taxonomía`).
3. Crea la tabla `table_catalog(table_name PK, grupo, label, sort_order)` vía migration `000147`.
4. Hace el **seed inicial** del catálogo con el mapeo (apuntando a los nombres ACTUALES de las tablas, no a los nombres propuestos).

Los renames (`verifications → tdd_verifications`, `sabotage_records → tdd_sabotage_records`, etc.) y los drops (billing, legacy) viven en HUs hermanas (42.5 renames SDD/TDD, 42.2/42.3 drops). `users`/`issues`/`roles`/`user_roles` quedan canónicas (NO se renombran). Esta HU es el **contrato** que esas HUs deben respetar.

## Convención

> **Toda tabla lleva el prefijo de su funcionalidad para poder agruparla.**

- El prefijo es `<grupo>_` (ej. `auth_`, `flow_`, `issue_`).
- Una tabla cuyo nombre coincide con su grupo en singular/plural puede mantenerse como **nombre canónico del grupo** (excepción documentada, estilo Rails/Postgres): p.ej. `users` ES el grupo `users`, `issues` ES el grupo `issue`. Ver `## Preguntas abiertas`.
- Tablas internas de tooling (`schema_migrations`) NO llevan prefijo y se **ocultan** del admin.
- El catálogo es la fuente de verdad: el admin `/database` lee `table_catalog` para agrupar/ordenar/etiquetar; cualquier tabla no listada se considera "sin clasificar" y se muestra al final.

## Mapa de taxonomía (23 grupos)

| Grupo | Prefijo | Label | Tablas (nombre actual) |
|---|---|---|---|
| users | `users_` | Usuarios y RBAC | users, roles, user_roles _(nombres canónicos del grupo — keep_ok, NO se renombran)_ |
| auth | `auth_` | Autenticación y credenciales | auth_sessions, auth_events, otp_codes, api_keys, secrets, invitations, org_enrollment_tokens |
| agent | `agent_` | Agentes | agents, agent_versions, agent_templates, agent_runs, agent_run_logs |
| flow | `flow_` | Flujos y orquestación | flows, flow_versions, flow_config, flow_runs, flow_run_steps, flow_run_step_snapshots, flow_signals, flow_run_signals_pending |
| skill | `skill_` | Skills | skills, skill_versions, skill_executions |
| mcp | `mcp_` | Servidores MCP | mcp_providers, mcp_servers, mcp_server_tools |
| prompt | `prompt_` | Prompts | prompts, captured_prompts |
| project | `project_` | Proyectos y tickets | projects, clients, project_templates, project_repositories, project_index_runs, project_merges, project_policies, project_policy_versions, project_tickets, project_ticket_comments, project_ticket_status_history, imported_workflow_files |
| sdd | `sdd_` | SDD (especificación dirigida) | requirements, proposals, designs |
| tdd | `tdd_` | TDD y verificación | verifications, verification_results, sabotage_records |
| issue | `issue_` | Issues / Historias de usuario | issues _(nombre canónico del grupo — keep_ok)_, issue_drafts, issue_draft_steps_log, gherkin_scenarios, tasks, code_references, intake_payloads |
| knowledge | `knowledge_` | Base de conocimiento | knowledge_docs, knowledge_chunks, observations |
| webhook | `webhook_` | Webhooks (entrada y salida) | webhooks, webhook_deliveries, outbound_webhook_subscriptions, outbound_webhook_deliveries |
| external | `external_` | Integraciones externas | external_providers, external_sync_state, external_sync_events |
| cron | `cron_` | Tareas programadas | crons, cron_executions |
| usage | `usage_` | Uso y cuotas | usage_counters, usage_alerts, usage_alert_fires |
| notification | `notification_` | Notificaciones | notification_deliveries |
| runner | `runner_` | Runners self-hosted | selfhosted_runners, selfhosted_tasks |
| platform | `platform_` | Políticas de plataforma | platform_policies, platform_policy_versions |
| file | `file_` | Archivos adjuntos | file_attachments |
| audit | `audit_` | Auditoría y actividad | audit_log, activity_log |
| seed | `seed_` | Seeders | seed_versions |
| internal | _(sin prefijo, oculto)_ | Interno (oculto) | schema_migrations |

**Nota sobre cobertura:** el mapa anterior cubre las tablas que se CONSERVAN (incluida `sabotage_records`, que se preserva como `tdd_`). Las tablas marcadas para DROP en HUs posteriores (billing: plans, budgets, cost_logs, cost_alerts_sent, org_cost_alert_thresholds — HU 42.2; legacy/infra: sessions, model_registry, entity_state_transitions, system_state, runtime_configs, dead_letter_queue, idempotency_keys, saga_compensation_log — HU 42.3) **NO se incluyen en el seed del catálogo** porque van a desaparecer. Quedan documentadas en `## Tablas fuera del catálogo` para trazabilidad.

## Tablas fuera del catálogo (no se siembran)

| Tabla | Motivo | HU destino |
|---|---|---|
| plans, budgets, cost_logs, cost_alerts_sent, org_cost_alert_thresholds | DROP (billing/costos; single-org sin facturación) | 42.2 |
| sessions | DROP (legacy); FKs entrantes `captured_prompts.session_id`, `verifications.session_id` (ON DELETE SET NULL) | 42.3 |
| model_registry | DROP (legacy; modelos resueltos en código) | 42.3 |
| entity_state_transitions | DROP (legacy; reemplazada por `*_status_history`) | 42.3 |
| system_state | DROP (legacy; reemplazada por flow_config/seed_versions) | 42.3 |
| runtime_configs | DROP (legacy; solapa con flow_config) | 42.3 |
| dead_letter_queue, idempotency_keys, saga_compensation_log | DROP (legacy/infra mensajería no usada) | 42.3 |

> **`sabotage_records` NO está fuera del catálogo:** se PRESERVA como capa TDD (mutation/sabotage testing, FK viva a `tasks`). Se siembra en el catálogo con su nombre ACTUAL `sabotage_records` (grupo `tdd`, label "TDD / Sabotaje") y se renombra a `tdd_sabotage_records` en la HU 42.5.

## Criterios de aceptación

```gherkin
Feature: Taxonomía y catálogo de tablas (source of truth)

  Background:
    Given la base de datos single-org con 97 tablas reales
    And la convención "toda tabla lleva prefijo de su funcionalidad"

  Scenario: La migration 000147 crea la tabla table_catalog
    When aplico la migration 000147 (up)
    Then existe la tabla "table_catalog"
    And tiene columnas: table_name (PK, text), grupo (text NOT NULL), label (text NOT NULL), sort_order (integer NOT NULL)
    And table_name es PRIMARY KEY

  Scenario: El seed inicial puebla el catálogo con los nombres ACTUALES
    Given la tabla table_catalog recién creada
    When la migration 000147 ejecuta el seed
    Then table_catalog tiene una fila por cada tabla conservada (nombre actual)
    And la fila de "otp_codes" tiene grupo "auth" y label "Autenticación y credenciales"
    And la fila de "verifications" tiene grupo "tdd"
    And la fila de "requirements" tiene grupo "sdd"
    And NO existe fila para "plans" (marcada para DROP)
    And NO existe fila para "sessions" (marcada para DROP)

  Scenario: Cada grupo tiene un sort_order coherente
    When consulto "SELECT DISTINCT grupo, sort_order FROM table_catalog ORDER BY sort_order"
    Then los grupos aparecen agrupados (todas las tablas de un grupo comparten rango de sort_order contiguo)
    And el orden refleja el orden del mapa de taxonomía (users primero, internal último)

  Scenario: schema_migrations queda fuera o marcada como oculta
    When consulto el catálogo
    Then "schema_migrations" NO aparece como grupo navegable del admin
    And se documenta que el admin la oculta

  Scenario: El catálogo NO renombra ni dropea ninguna tabla
    Given el conteo de tablas reales ANTES de la migration
    When aplico 000147 (up)
    Then el conteo de tablas reales aumenta en exactamente 1 (la nueva table_catalog)
    And ninguna tabla preexistente cambió de nombre
    And ninguna tabla preexistente fue eliminada

  Scenario: La migration es reversible
    Given la migration 000147 aplicada
    When aplico 000147 (down)
    Then la tabla table_catalog deja de existir
    And el resto del schema queda intacto

  Scenario: El seed es idempotente bajo reaplicación
    Given table_catalog ya poblada
    When el seed se ejecuta de nuevo (ON CONFLICT DO UPDATE)
    Then no se duplican filas
    And el label/grupo/sort_order se actualizan al valor canónico
```

## Análisis SDD/TDD de la taxonomía

EL DOMINIO SDD ESTÁ COMPLETO Y BIEN FORMADO; el TDD ESTÁ CUBIERTO pero por tablas con nombres no-obvios (no hay tablas `tests_`/`coverage_` porque la arquitectura NO las necesita).

**Pipeline SDD verificado** (migrations 000050-000054, 000058, 000111):

```
requirements (REQ raíz, jerárquico parent_id)
  └─> issues (ex user_stories/HU, FK req_id → requirements, slug unique)
        ├─> gherkin_scenarios (criterios de aceptación BDD: feature/scenario/given/when/then, FK issue_id)
        ├─> proposals (versionado por issue)
        │     └─> designs (FK proposal_id + issue_id, versionado)
        ├─> tasks (FK issue_id, section + position)
        │     └─> verification_results (FK task_id: result pass/fail, evidence, notes)
        └─> code_references (FK issue_id: archivos tocados)
intake_payloads  → alimenta el pipeline (source: agent/email/webhook/slack/sheet/manual),
                   commitea hacia issues/requirements (committed_hu_id / committed_req_id)
issue_drafts + issue_draft_steps_log → wizard conversacional que construye un issue antes de materializarlo
```

**¿Hay capa TDD? SÍ, distribuida en 3-4 tablas:**

1. `gherkin_scenarios` = especificación de tests (BDD/acceptance), el contrato de comportamiento. Por eso pertenece al issue → `issue_gherkin_scenarios` (NO `sdd_`/`tdd_`).
2. `verification_results` = evidencia de ejecución POR TAREA (result, evidence, notes, FK task_id). Registro TDD persistente → `tdd_verification_results`.
3. `verifications` (migration 000111, REQ-50) = checkpoints lightweight (build/test/lint/smoke/typecheck/migration, items JSONB) que el LLM dispara tras un cambio; el server NO ejecuta, solo guarda → `tdd_verifications`. **ATENCIÓN:** la migration original trae RLS + organization_id; verificar single-org antes de renombrar constraints/policy (esa verificación es de la HU de rename, no de esta).
4. `sabotage_records` (migration 000053, FK task_id) = mutation/sabotage testing ("rompé el código a propósito y confirmá que el test FALLA": action/expected_failure/actual_result/restored). ES TDD REAL. **DECISIÓN del usuario: se PRESERVA** como parte del flujo SDD/capa TDD. Se siembra en el catálogo con su nombre actual `sabotage_records` (grupo `tdd`) y se renombra a `tdd_sabotage_records` en la HU 42.5.

**¿Faltan tablas `sdd_`/`tdd_`? NO se justifica inventar ninguna:**

- NO hace falta `tdd_test_cases` ni `tdd_coverage`: el server es **stateless** respecto a la ejecución (el LLM corre los tests con Bash/Read nativos y reporta vía `verifications`). Crear coverage/test_cases violaría esa decisión arquitectónica.
- NO hace falta `sdd_acceptance_criteria` separado: `gherkin_scenarios` YA cumple ese rol.
- El único hueco DEFENDIBLE (no obligatorio) sería una tabla puente issue↔ticket, pero `project_tickets.linked_issue_id` YA lo resuelve. No se crea nada.

**Conclusión:** dominio SDD completo; TDD completo pero hay que hacerlo VISIBLE renombrando `verifications`/`verification_results`/`sabotage_records` a `tdd_*` (en HU 42.5). `gherkin` se queda en `issue_*` por pertenencia. `sabotage_records` se PRESERVA como `tdd_sabotage_records` (decisión confirmada del usuario).

## Análisis breve

- **Qué pide realmente:** fijar el contrato de naming y materializarlo en una tabla consultable, sin tocar el schema vivo. Es la fundación de todo REQ-42.
- **Módulos a tocar:** SOLO `internal/migrate/migrations/000147_*` (up+down). El consumo en el admin `/database` es de HUs posteriores.
- **Riesgos / dependencias:** el seed debe apuntar a nombres ACTUALES (pre-rename) para no romper; si una HU de rename corre antes que el catálogo se actualice, el catálogo queda desincronizado (mitigado: las HUs de rename incluyen el `UPDATE table_catalog SET table_name = ...`).
- **Esfuerzo tentativo:** S

## Preguntas abiertas

1. **users / issues:** RESUELTO — `users`, `issues`, `roles` y `user_roles` quedan como NOMBRE CANÓNICO del grupo (excepción documentada estilo Rails/Postgres). NO se renombran (action=keep_ok). El seed usa el nombre ACTUAL.
2. **roles/user_roles bajo `users_` vs `auth_`:** elegido `users_` (autorización = identidad). Quedan canónicas (sin prefijo redundante), agrupadas en `users`.
3. **enrollment_tokens:** se pidió quitar `org_` y dejar `enrollment_tokens`. ¿Sin prefijo de grupo o agrupado como `auth_enrollment_tokens`? Hay tensión con la regla. **El seed usa el nombre actual `org_enrollment_tokens`.**
4. **sabotage_records:** RESUELTO — se PRESERVA como `tdd_sabotage_records` (capa TDD, mutation testing). El seed usa el nombre actual `sabotage_records`; el rename es 42.5.
5. **saga_compensation_log:** RESUELTO — se DROPEA (cluster saga/infra, HU 42.3). NO se siembra en el catálogo.
6. **DROP de sessions:** FKs entrantes (`captured_prompts.session_id`, `verifications.session_id`, ON DELETE SET NULL). ¿Limpiamos esas columnas en la misma migración de drop?
7. **captured_prompts:** ¿rename a `prompt_captured` o keep? Solo naming.
8. **outbound_webhook_*:** ¿reordenar a `webhook_outbound_*` o mantener?
9. **clients / observations:** agrupados bajo `project_` y `knowledge_`. ¿OK o grupos propios (`client_`, `memory_`)?

## Verificación previa

- [ ] Confirmar que la próxima migration libre es `000147` (última aplicada: 000146)
- [ ] Confirmar el conteo real de tablas (esperado: 97) vía introspección del schema
- [ ] Confirmar que NO existe ya una tabla `table_catalog`
- [ ] Confirmar que los nombres del seed coinciden EXACTAMENTE con los nombres actuales en el schema real (no con los propuestos)
- [ ] Confirmar que el seed NO incluye las tablas marcadas para DROP
- [ ] Confirmar que la migration pasa `squawk` (DDL aditivo, sin locks peligrosos)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
