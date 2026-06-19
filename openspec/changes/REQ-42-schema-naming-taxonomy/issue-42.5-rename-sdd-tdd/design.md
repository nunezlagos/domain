# Design: issue-42.5-rename-sdd-tdd

## Decisión arquitectónica

**Rename atómico en una sola transacción `BEGIN/COMMIT`, patrón 000146 + 000073.** Todos los `ALTER TABLE ... RENAME TO`, `ALTER INDEX ... RENAME TO` y `ALTER TABLE ... RENAME CONSTRAINT` de las 11 tablas viven en la MISMA tx. Esto es obligatorio porque hay FKs *entre* tablas renombradas en el mismo lote (`designs.proposal_id → proposals`, `verification_results.task_id → tasks`): aunque Postgres mantiene la FK por OID (el rename de la tabla referenciada NO rompe la referencia), ejecutar todo junto garantiza que un fallo parcial no deje el schema a medias.

**Los RENAME son metadata-only.** No reescriben filas ni reconstruyen índices; el lock es breve y squawk no debería objetar. Estimado < 2s.

**Se aprovecha el rename para alinear nombres legacy.** La migración 000073 renombró las COLUMNAS `hu_id → issue_id` / `committed_hu_id → committed_issue_id`, pero NO renombró los índices/constraints que las nombraban. Por eso hoy existen objetos como `proposals_hu_id_version_key`, `designs_hu_id_fkey`, `code_references_hu_id_idx`, `intake_payloads_committed_hu_id_fkey`, `gherkin_hu_id_idx`, `tasks_hu_id_fkey`. Este lote los re-prefija Y los pasa a `issue_id` de una vez. Igual para `verifications`: los índices conservan `org` en el nombre (`verifications_org_project_status_idx`, `verifications_org_session_idx`) pese a que la columna `organization_id` fue dropeada en 000132 → se les quita el `org`.

**Sin sequences.** Las 11 tablas no tienen sequence que renombrar: las 10 SDD/issue/TDD usan `UUID PK` con `gen_random_uuid()`, y `sabotage_records` tiene `id` no-serial (verificado en introspección: no aparece `sabotage_records_id_seq` en `::SEQUENCES::`). A diferencia de 000146 (que tenía `org_flow_config_id_seq`), aquí NO hay `ALTER SEQUENCE`.

**Triggers — regla y excepción:**
- `trg_set_updated_at` es GENÉRICO (sin sufijo de tabla). Los triggers viven en la tabla, no son globales, así que el RENAME de la tabla los arrastra solo. **NO se renombra.**
- `intake_payloads` tiene ADEMÁS `set_updated_at_intake_payloads` (sufijo legacy de tabla). No se rompe con el RENAME (sigue colgado de la tabla), pero por consistencia se hace `DROP TRIGGER ... + CREATE TRIGGER set_updated_at_issue_intake_payloads` (patrón 000073 líneas 27-35). El genérico se queda.

**RLS:** ninguna de las 11 tablas tiene policy vigente (sólo `audit_log` y `otp_codes`). `verifications` tuvo RLS+`organization_id` (000111) pero 000132 hizo `DISABLE ROW LEVEL SECURITY` y dropeó la columna; queda `relforcerowsecurity=true` inerte (sin policy no tiene efecto). Se limpia con `ALTER TABLE tdd_verifications NO FORCE ROW LEVEL SECURITY` (cosmético, no rompe nada). **NO se renombra ninguna policy porque no hay ninguna.**

## DDL — bloque representativo del up (000151)

```sql
BEGIN;

-- SDD: requirements → sdd_requirements
ALTER TABLE IF EXISTS requirements RENAME TO sdd_requirements;
ALTER INDEX IF EXISTS requirements_pkey          RENAME TO sdd_requirements_pkey;
ALTER INDEX IF EXISTS requirements_parent_id_idx RENAME TO sdd_requirements_parent_id_idx;
-- ... priority/slug/status idx ...
ALTER TABLE sdd_requirements RENAME CONSTRAINT requirements_parent_id_fkey TO sdd_requirements_parent_id_fkey;

-- SDD: proposals → sdd_proposals (alinea hu_id → issue_id)
ALTER TABLE IF EXISTS proposals RENAME TO sdd_proposals;
ALTER INDEX IF EXISTS proposals_hu_id_version_key RENAME TO sdd_proposals_issue_id_version_key;
ALTER TABLE sdd_proposals RENAME CONSTRAINT proposals_hu_id_fkey TO sdd_proposals_issue_id_fkey;

-- TDD: verifications → tdd_verifications (quita 'org' residual)
ALTER TABLE IF EXISTS verifications RENAME TO tdd_verifications;
ALTER INDEX IF EXISTS verifications_org_project_status_idx RENAME TO tdd_verifications_project_status_idx;
ALTER TABLE tdd_verifications NO FORCE ROW LEVEL SECURITY;

-- ISSUE: intake_payloads → issue_intake_payloads (recrea trigger legacy)
ALTER TABLE IF EXISTS intake_payloads RENAME TO issue_intake_payloads;
DROP TRIGGER IF EXISTS set_updated_at_intake_payloads ON issue_intake_payloads;
CREATE TRIGGER set_updated_at_issue_intake_payloads
  BEFORE UPDATE ON issue_intake_payloads
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- TDD: sabotage_records → tdd_sabotage_records (sin sequence; FK → issue_tasks)
ALTER TABLE IF EXISTS sabotage_records RENAME TO tdd_sabotage_records;
ALTER INDEX IF EXISTS sabotage_task_id_idx RENAME TO tdd_sabotage_records_task_id_idx;
ALTER TABLE tdd_sabotage_records RENAME CONSTRAINT sabotage_records_task_id_fkey TO tdd_sabotage_records_task_id_fkey;

-- ... resto de las 11 tablas ...

-- GRANT defensivo (app_user / app_admin / app_readonly) con EXCEPTION WHEN OTHERS THEN NULL

COMMIT;
```

El archivo completo está en `services/domain-backend/internal/migrate/migrations/000151_rename_sdd_tdd_issue_taxonomy.{up,down}.sql`. El `down` invierte cada bloque en orden inverso (primero TDD, último SDD) y restaura nombres legacy `*_hu_id_*` + el trigger `set_updated_at_intake_payloads`.

## Mapa de FKs (qué se mueve y qué sobrevive solo)

| FK | Apunta a | Acción |
|---|---|---|
| `sdd_designs.proposal_id` | `proposals` → `sdd_proposals` | Constraint re-prefijado; referencia por OID intacta (misma tx) |
| `tdd_verification_results.task_id` | `tasks` → `issue_tasks` | Constraint re-prefijado; referencia por OID intacta (misma tx) |
| `sdd_proposals.issue_id` | `issues` (no se renombra acá) | Sólo se re-prefija el constraint; FK intacta |
| `sdd_designs.issue_id` | `issues` | Sólo se re-prefija el constraint |
| `issue_gherkin_scenarios.issue_id` | `issues` | Sólo se re-prefija el constraint |
| `issue_tasks.issue_id` | `issues` | Sólo se re-prefija el constraint |
| `issue_code_references.issue_id` | `issues` | Sólo se re-prefija el constraint |
| `issue_intake_payloads.committed_issue_id` | `issues` | Constraint `committed_hu_id_fkey` → `committed_issue_id_fkey` |
| `issue_intake_payloads.committed_req_id` | `requirements` → `sdd_requirements` | Referencia por OID intacta (misma tx); constraint NO cambia de columna |
| `sdd_requirements.parent_id` | self (`sdd_requirements`) | Self-FK re-prefijado |
| `tdd_sabotage_records.task_id` | `tasks` → `issue_tasks` | Tabla renombrada en este lote; constraint `sabotage_records_task_id_fkey` → `tdd_sabotage_records_task_id_fkey`; referencia por OID intacta (misma tx) |

## Propuesta tablas faltantes SDD/TDD (NO se crean acá — solo propuesta)

Análisis de huecos del dominio. **Conclusión: el dominio SDD está completo y el TDD también; no se justifica inventar tablas.** Detalle de cada candidata evaluada:

| Tabla candidata | Veredicto | Razón |
|---|---|---|
| `tdd_test_cases` | **NO crear** | El server es **stateless respecto a la ejecución de tests**. El LLM corre los tests con Bash/Read nativos y reporta el resultado vía `items` (JSONB) de `tdd_verifications`. Persistir casos de test en DB violaría esa decisión arquitectónica (el server no ejecuta nada). |
| `tdd_coverage` | **NO crear** | Mismo motivo: la cobertura la calcula la herramienta que corre los tests (no el server). Si hiciera falta histórico, va como campo en `items` JSONB, no como tabla. |
| `sdd_acceptance_criteria` | **NO crear** | `issue_gherkin_scenarios` (feature/scenario/given/when/then) YA es la especificación de aceptación BDD. Crear otra tabla duplicaría el rol. |
| Puente `issue ↔ ticket` | **NO crear** | `project_tickets.linked_issue_id` ya resuelve la traza issue→ticket. |
| `tdd_sabotage_records` (rename de `sabotage_records`) | **SE INCLUYE en 000151** | `sabotage_records` (000053, FK `task_id`, campos `action`/`expected_failure`/`actual_result`/`restored`) ES mutation/sabotage testing real — un pilar TDD legítimo. DECISIÓN del usuario: se PRESERVA y se renombra a `tdd_sabotage_records` en este lote (no es DROP). |

El rename de `sabotage_records` → `tdd_sabotage_records` sigue el mismo patrón (pkey + status_idx + `sabotage_task_id_idx` → `tdd_sabotage_records_task_id_idx` + `task_id_fkey` → `tdd_sabotage_records_task_id_fkey`), SIN sequence (`id` no-serial), y es compatible con el rename `tasks → issue_tasks` de este lote (FK por OID).

## TDD plan (migración)

1. **Red:** test de integración que aplica 000151 up sobre una DB de prueba y verifica con `to_regclass('sdd_requirements') IS NOT NULL` y `to_regclass('requirements') IS NULL` para las 10 tablas.
2. **Green:** la migración 000151 up.
3. **Red:** test que verifica que `tdd_verification_results.task_id` referencia `issue_tasks` (consultar `information_schema.referential_constraints` / `pg_constraint`).
4. **Green:** orden de renames en la misma tx (ya está).
5. **Red:** test que `SELECT count(*) FROM pg_indexes WHERE indexname LIKE '%\_hu\_id\_%'` para estas tablas = 0 tras el up.
6. **Green:** los `ALTER INDEX RENAME` que alinean `hu_id → issue_id`.
7. **Red:** test de rollback — aplicar up + down y verificar que vuelve al estado original (incluido el trigger `set_updated_at_intake_payloads`).
8. **Sabotaje:** ver tasks.md → quitar un solo `ALTER ... RENAME CONSTRAINT` y confirmar que el test de FK/constraint FALLA (no debe pasar en verde con un constraint legacy sin renombrar).

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Falsos positivos al renombrar `tasks` (palabra común en Go; `selfhosted_tasks` es OTRA tabla) | Filtrar a literales SQL con `FROM/INTO/UPDATE/JOIN/DELETE FROM`. Tocar SOLO strings de query en `service/task/service.go` y `traceability`. NUNCA tocar identificadores Go. |
| Migración aplica pero el código viejo sigue corriendo → `relation "requirements" does not exist` | **Deploy atómico**: migración + binario actualizado en el mismo release. tasks.md lista TODOS los touchpoints. |
| FK entre tablas renombradas se rompe | Mantener todo en UNA tx; Postgres preserva la FK por OID. Verificado en precedente 000073. |
| Constraint con nombre legacy queda sin renombrar (silencioso) | Test de sabotaje + assert `pg_constraint` sin `_hu_id_` ni `_org_`. |
| El trigger genérico `trg_set_updated_at` se renombra por error y rompe el `updated_at` | Regla explícita: NO tocar el genérico; sólo el `set_updated_at_intake_payloads` legacy. |
| `verifications` todavía tuviera RLS/organization_id (rompe el rename de policy inexistente) | Verificado contra VPS: 000132 dropeó la columna y la policy; el up sólo hace `NO FORCE` (idempotente, inerte). |
| squawk objeta el rename | Los RENAME son metadata-only (sin reescritura, sin lock largo). Correr `squawk` en el cierre; si objeta, documentar el warning. |
| `seed_demo.go` inserta en `verifications` con el nombre viejo tras el rename | Actualizar el seeder a `tdd_verifications` en el mismo lote (touchpoint listado). |
