# REQ-42-schema-naming-taxonomy: Taxonomía de naming por funcionalidad para las 97 tablas del schema, limpieza de tablas redundantes y agrupamiento del explorador de base de datos en el admin.

**Estado:** activo
**Creado:** 2026-06-18
**Fase:** F4

## Descripción

El schema de `domain-backend` creció sin una convención de naming consistente: hay tablas con prefijo de dominio (`flow_*`, `agent_*`, `project_*`), tablas sueltas sin prefijo (`requirements`, `proposals`, `designs`, `tasks`, `gherkin_scenarios`, `verifications`, `observations`, `clients`), prefijos legacy del diseño multi-tenant (`org_enrollment_tokens`, `org_cost_alert_thresholds`) y tablas redundantes que ya no se usan en el despliegue single-org (billing/costos e infra de mensajería).

REQ-42 fija una **taxonomía oficial**: TODA tabla lleva el prefijo de su funcionalidad para poder agruparla (`auth_`, `users_`, `agent_`, `flow_`, `skill_`, `mcp_`, `prompt_`, `project_`, `sdd_`, `tdd_`, `issue_`, `knowledge_`, `webhook_`, `external_`, `cron_`, `usage_`, `notification_`, `runner_`, `platform_`, `file_`, `audit_`, `seed_`). La taxonomía se materializa en una tabla `table_catalog` consultable (source of truth), se aplican los **drops** de billing/costos y legacy/infra para reducir la superficie, se crea una tabla nueva `agent_run_prompts` (captura del prompt efectivo por iteración de run), se ejecutan los **renames por dominio** en migraciones atómicas, el explorador `/database` del admin Angular se reescribe para **agrupar por prefijo** (con `schema_migrations` oculta y `seed_versions` visible como "Seeders corridos"), y finalmente el linter de migraciones (`internal/dbconvlint`) pasa a **enforcing** del prefijo para que ninguna `CREATE TABLE` futura escape a la convención.

La base está casi vacía (solo `auth_events`, `auth_sessions`, `schema_migrations` tienen filas), por lo que los renames y drops tienen riesgo NULO sobre datos. Se usa **RENAME directo single-org** (NO expand/contract), siguiendo el precedente atómico de la migración 000146 (`org_flow_config` → `flow_config`): `ALTER TABLE RENAME` + `ALTER SEQUENCE RENAME` + `ALTER INDEX RENAME` + `ALTER TABLE RENAME CONSTRAINT`, todo en una sola transacción `BEGIN/COMMIT`. Cada migración de rename va en el MISMO deploy que su cambio de código Go (aplicar la migración con el binario viejo rompe el pipeline con `relation ... does not exist`).

## Decisiones arquitectónicas fijadas

- **Prefijo por funcionalidad**: toda tabla lleva `<grupo>_` para poder agruparla en el admin. Excepción documentada y CONFIRMADA: una tabla cuyo nombre coincide con su grupo se mantiene como **nombre canónico del grupo** estilo Rails/Postgres. Aplica a `users`, `issues`, `roles` y `user_roles` (grupo `users`): NO se renombran. Tablas internas de tooling (`schema_migrations`) NO llevan prefijo y se ocultan.
- **Catálogo como source of truth**: `table_catalog(table_name PK, grupo, label, sort_order)` gobierna el agrupamiento/orden/etiquetado del admin. Se siembra con los nombres ACTUALES (pre-rename); cada HU de rename actualiza el catálogo en su misma migración (`UPDATE table_catalog SET table_name = ...`).
- **RENAME directo single-org (NO expand/contract)**: la base está casi vacía y es single-org. Se renombra en una sola transacción atómica arrastrando índices, sequences, constraints y RLS policies. Sin fase de doble escritura ni columnas espejo.
- **Migración + código en el mismo deploy**: cada rename de tabla va acompañado de los touchpoints Go (queries SQL embebidas) en el mismo deploy. Aplicar la migración con el binario viejo (o al revés) rompe el pipeline.
- **Drops de billing/costos**: el modelo es **free total, single-org, sin facturación**. Se dropean `plans`, `budgets`, `cost_logs`, `cost_alerts_sent`, `org_cost_alert_thresholds` junto con su código Go. La observabilidad de producto queda en el grupo `usage_` (métrica de uso, NO costo).
- **Drops de legacy/infra**: se dropean 8 tablas que no encajan en ningún grupo (`sessions`, `model_registry`, `entity_state_transitions`, `system_state`, `saga_compensation_log`, `runtime_configs`, `dead_letter_queue`, `idempotency_keys`). Al dropear `sessions` se limpian las FKs entrantes (`captured_prompts.session_id`, `verifications.session_id`, ON DELETE SET NULL: DROP CONSTRAINT + DROP COLUMN explícitos) en la misma migración. `sabotage_records` NO se dropea: se preserva como capa TDD (rename a `tdd_sabotage_records` en 42.5).
- **Tabla nueva `agent_run_prompts`**: cada iteración de un `agent_run` persiste el PROMPT EFECTIVO que la plataforma arma y manda al LLM (system prompt resuelto + mensajes ensamblados + tools expuestas). Cubre observabilidad/auditoría de lo que realmente recibió el modelo. Ya nace con prefijo `agent_` correcto.
- **TDD visible**: la capa de verificación TDD existe pero estaba escondida en nombres no-obvios. `verifications` → `tdd_verifications`, `verification_results` → `tdd_verification_results`, `sabotage_records` → `tdd_sabotage_records` (mutation/sabotage testing, preservada). `gherkin_scenarios` se queda en `issue_*` (pertenece al issue, es la especificación de aceptación). NO se inventan tablas `tdd_test_cases`/`tdd_coverage`: el server es stateless respecto a la ejecución (el LLM corre los tests y reporta vía `verifications`).
- **Lint al final (enforcing a futuro)**: `internal/dbconvlint` rechaza toda `CREATE TABLE` sin prefijo válido de la taxonomía (salvo nombres canónicos documentados). Se activa al final para no bloquear las propias migraciones de REQ-42 y para enforce a partir de ahí.

## Criterios de éxito

- Existe `table_catalog` con una fila por cada tabla conservada, con grupo/label/sort_order coherentes; el admin la usa como única fuente de verdad
- Toda tabla conservada lleva el prefijo de su grupo funcional (salvo los nombres canónicos `users`/`issues`/`roles`/`user_roles`, y `schema_migrations` interno)
- Las 5 tablas de billing/costos y las 8 tablas legacy/infra ya NO existen, y el código Go que las consultaba fue removido/refactorizado en el mismo deploy
- Existe `agent_run_prompts` y cada iteración de un `agent_run` persiste el prompt efectivo enviado al LLM
- El pipeline SDD/TDD funciona de punta a punta con los nombres nuevos (intake → req → issue → gherkin → proposal → design → task → verification) sin errores de relación
- El explorador `/database` del admin muestra las tablas agrupadas por funcionalidad derivada del prefijo, con `schema_migrations` oculta y `seed_versions` visible bajo "Seeders corridos"
- El linter de migraciones rechaza toda `CREATE TABLE` cuyo nombre no empiece con un prefijo válido de la taxonomía
- Cada migración de rename es atómica (`BEGIN/COMMIT`) y reversible (`down` que restaura nombres legacy), arrastrando índices/sequences/constraints/RLS
- Los tests de sabotaje pasan (romper UN rename → el test cae → restaurar → vuelve a verde): los tests detectan realmente un rename incompleto, no pasan por casualidad
- `go vet`, `go build`, `go test` y `squawk` verdes; grep final de los nombres legacy en `internal/`, `cmd/`, `tests/` da 0 resultados

## HUs hijas

| HU | Estado | Migración | Descripción |
|----|--------|-----------|-------------|
| issue-42.1-taxonomia-y-catalogo | propuesta | 000147 | Fija la convención de naming y crea + siembra `table_catalog` (source of truth) con los nombres ACTUALES. NO renombra ni dropea nada. |
| issue-42.2-drop-billing-costos | propuesta | 000148 | Dropea el dominio billing/costos (`plans`, `budgets`, `cost_logs`, `cost_alerts_sent`, `org_cost_alert_thresholds`) y el código Go que las consulta. |
| issue-42.3-drop-legacy-infra | propuesta | 000149 | Dropea 8 tablas legacy/infra (`sessions`, `model_registry`, `entity_state_transitions`, `system_state`, `saga_compensation_log`, `runtime_configs`, `dead_letter_queue`, `idempotency_keys`) + limpia FKs de `sessions` + refactoriza el código. (`sabotage_records` NO se dropea: se preserva → 42.5.) |
| issue-42.4-tabla-agent-run-prompts | propuesta | 000150 | Crea `agent_run_prompts`: persiste el prompt efectivo (system resuelto + mensajes ensamblados + tools) por iteración de `agent_run`. |
| issue-42.5-rename-sdd-tdd | propuesta | 000151 | Renombra el pipeline SDD/TDD + capa issue: `requirements`→`sdd_requirements`, `proposals`→`sdd_proposals`, `designs`→`sdd_designs`, `verifications`→`tdd_verifications`, `verification_results`→`tdd_verification_results`, `sabotage_records`→`tdd_sabotage_records`, `tasks`→`issue_tasks`, `code_references`→`issue_code_references`, `intake_payloads`→`issue_intake_payloads`. |
| issue-42.6-rename-issues | propuesta | 000152 | Renombra `gherkin_scenarios`→`issue_gherkin_scenarios` con índices/constraints alineados a `issue_*`. |
| issue-42.7-rename-enrollment | propuesta | 000153 | Renombra `org_enrollment_tokens`→`enrollment_tokens` (saca el prefijo `org_` legacy multi-tenant). |
| issue-42.8-rename-auth-users | propuesta | 000154 | Renombra el grupo AUTH (`otp_codes`→`auth_otp_codes`, `api_keys`→`auth_api_keys`, `secrets`→`auth_secrets`, `invitations`→`auth_invitations`, `org_enrollment_tokens`→`enrollment_tokens`) arrastrando índices/constraints/RLS. `users`/`roles`/`user_roles` quedan canónicas (NO se renombran). |
| issue-42.9-rename-resto | propuesta | 000155 | Renombra el resto con `action=rename` no cubierto antes: `clients`→`project_clients`, `imported_workflow_files`→`project_imported_workflow_files`, `captured_prompts`→`prompt_captured`, `observations`→`knowledge_observations`, `outbound_webhook_*`→`webhook_outbound_*`, `selfhosted_*`→`runner_selfhosted_*`, `activity_log`→`audit_activity_log`. |
| issue-42.10-angular-grouping-database | propuesta | — | Reescribe el explorador `/database` del admin Angular para agrupar por funcionalidad (prefijo real), con `schema_migrations` oculta y `seed_versions` visible bajo "Seeders corridos". |
| issue-42.11-lint-enforce-prefix | propuesta | — | El linter `internal/dbconvlint` rechaza toda `CREATE TABLE` sin prefijo válido de la taxonomía (salvo nombres canónicos documentados). Enforcing a futuro. |

## Dependencias

- REQ-04-opsx-sdd (pipeline SDD) — **implementado**. Provee `requirements`/`proposals`/`designs`/`issues`/`tasks`/`gherkin_scenarios`/`verifications` que esta HU renombra a `sdd_*`/`tdd_*`/`issue_*`. El rename debe preservar todas las FKs del pipeline.
- REQ-41-admin-dashboard (panel admin) — **propuesto**. El explorador `/database` (HU 42.10) vive en el mismo `services/domain-admin` que REQ-41; reusa el patrón de vistas standalone + signals + HttpClient.
- Precedente de rename atómico: migración 000146 (`org_flow_config` → `flow_config`) — **aplicado**. Define el patrón `ALTER TABLE/SEQUENCE/INDEX RENAME` + `RENAME CONSTRAINT` en una sola transacción.
- `internal/dbconvlint` (linter de convenciones de migración) — base existente que HU 42.11 extiende a enforcing del prefijo.

## No-objetivos (fuera de alcance)

- Migrar a multi-tenant o reintroducir `organization_id` / RLS por org (el despliegue es single-org; los prefijos `org_` legacy se ELIMINAN, no se generalizan)
- Reintroducir billing/planes/tiers/invoices (modelo free total; las tablas de costos se DROPean, no se renombran)
- Cambiar nombres de columnas, tipos, o el contenido semántico de las tablas (REQ-42 es naming + drops + 1 tabla nueva, NO un rediseño de datos)
- Estrategia expand/contract con doble escritura (innecesaria en single-org casi vacío; se usa RENAME directo atómico)
- Reescribir el cliente/SDK TS o los endpoints públicos del API por el rename (los nombres de tabla son internos; el contrato HTTP no cambia)
- Resolver el desajuste preexistente de `tests/e2e/schema_audit_test.go` con tablas inexistentes (`organizations`, `intake_attachments`, `custom_roles`, `event_log`) — se deja un TODO; la limpieza es de otra HU
- Crear tablas `tdd_test_cases`/`tdd_coverage` (el server es stateless respecto a la ejecución de tests)

## Preguntas abiertas

1. **users / issues**: RESUELTO — `users`, `issues`, `roles`, `user_roles` quedan como NOMBRE CANÓNICO del grupo (excepción estilo Rails/Postgres). NO se renombran.
2. **roles / user_roles bajo `users_` vs `auth_`**: RESUELTO — grupo `users` (autorización = identidad), canónicas sin prefijo redundante.
3. **enrollment_tokens**: se pidió quitar `org_` y dejar `enrollment_tokens` literal. ¿Sin prefijo de grupo o agrupado como `auth_enrollment_tokens`? Tensión con la regla "toda tabla lleva prefijo".
4. **sabotage_records**: RESUELTO — se PRESERVA como `tdd_sabotage_records` (mutation/sabotage testing, capa TDD). Rename en 42.5; NO drop.
5. **saga_compensation_log**: RESUELTO — se DROPEA (cluster saga/infra) en 42.3, en lugar de `sabotage_records`.
6. **DROP de sessions**: tiene FKs entrantes (`captured_prompts.session_id`, `verifications.session_id`, ON DELETE SET NULL). Se limpian esas columnas en la misma migración de drop. Confirmar.
7. **captured_prompts**: ¿rename a `prompt_captured` (agrupar) o keep? Solo naming, el dato se conserva.
8. **outbound_webhook_***: ¿reordenar `outbound_` a sufijo (`webhook_outbound_subscriptions`/`_deliveries`) o mantener nombres actuales?
9. **verifications → tdd_***: la migración 000111 traía RLS + `organization_id`. Confirmar que ya está en single-org (RLS removida en 000132) antes de renombrar constraints/policy.
10. **clients / observations**: agrupados bajo `project_` y `knowledge_`. ¿De acuerdo o preferís grupos propios (`client_`, `memory_`)?

## Orden de implementación

Ver `implementation-order.md` en esta misma carpeta. 6 olas: catálogo → drops → tabla nueva → renames por dominio → Angular → lint.
