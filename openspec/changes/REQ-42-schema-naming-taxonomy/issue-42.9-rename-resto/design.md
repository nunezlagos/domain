# Design: issue-42.9-rename-resto

## Decisión arquitectónica

**Rename atómico directo (NO expand/contract), una sola transacción `BEGIN/COMMIT`, replicando el precedente de la migration 000146.** El sistema es single-org y la DB está casi vacía (solo `auth_events`, `auth_sessions`, `schema_migrations` tienen filas; las 9 tablas objetivo tienen 0 rows). El riesgo de datos es NULO. No se justifica un esquema expand/contract (crear tabla nueva + copiar + dual-write + drop): para un rename puro de identificador, `ALTER TABLE RENAME` es atómico, instantáneo (`<1s`) y preserva PK, índices, constraints y FKs (las FKs se resuelven por OID, no por nombre).

**De-scope de colisiones (verificado contra HU hermanas):** el `renMisc` original traía 12 tablas, pero tres ya tienen dueño en otras HU de REQ-42 — `org_enrollment_tokens → enrollment_tokens` (HU 42.7, migration 000153), `verifications → tdd_verifications` y `verification_results → tdd_verification_results` (HU 42.5, migration 000151). Para evitar doble rename de la misma tabla, esta HU 42.9 las EXCLUYE y cubre solo las **9 restantes**.

**Por qué todo en una sola migration (000155) y una sola transacción:** los pares con FK interna (`webhook_outbound_subscriptions`↔`deliveries`, `runner_selfhosted`↔`tasks`) deben renombrarse juntos para no dejar el schema en estado intermedio inconsistente entre pasos. El resto de tablas son independientes pero se agrupan en la misma tx por coherencia y para que el rollback sea un único punto.

**Si NO quedan renames reales (no-op):** si al momento de implementar, otra HU de REQ-42 ya absorbió estos renames, esta HU NO emite la migration 000155. En ese caso `design.md` declara: *"No requiere migration — el resto de tablas ya cumple prefijo (keep_ok). La HU queda como verificación documental."* El Gherkin de verificación de `issue.md` (último scenario) sigue siendo el criterio de cierre.

## Reglas de rename de objetos (verificadas contra Postgres)

| Objeto | Comando | Trampa |
|---|---|---|
| Tabla | `ALTER TABLE <old> RENAME TO <new>` | Renombra la tabla; FKs entrantes/salientes se preservan por OID |
| Índice | `ALTER INDEX <old> RENAME TO <new>` | Renombra el índice; NO cambia el método (btree/gin/ivfflat) ni el predicado WHERE de índices parciales |
| Constraint CHECK / FK | `ALTER TABLE <new> RENAME CONSTRAINT <old> TO <new>` | Solo para CHECK y FK puras |
| pkey | **solo `ALTER INDEX`** | pkey existe como índice Y constraint pero **comparten objeto**. `ALTER INDEX` renombra ambos. Emitir además `RENAME CONSTRAINT` sobre el pkey → **error por duplicado** |
| UNIQUE con índice homónimo | **solo `ALTER INDEX`** | Misma regla que pkey: `imported_workflow_files_unique` es constraint con índice del mismo nombre → un solo `ALTER INDEX` |
| Sequence | N/A | Ninguna de las 9 tablas tiene sequence (todas PK UUID) |
| RLS policy | N/A | Ninguna de las 9 tiene policy activa (`relrowsecurity=f`). `activity_log`/`observations`/etc tienen `relforcerowsecurity=t` pero **FORCE sin ENABLE es inerte** y no hay policy listada |

## DDL de la migration 000155 (up)

Estructura de la transacción (orden: independientes primero, pares con FK juntos):

```sql
BEGIN;

-- (DE-SCOPE) enrollment_tokens lo hace HU 42.7 (000153); verifications y
-- verification_results los hace HU 42.5 (000151). NO van en esta migration.

-- 1) captured_prompts → prompt_captured
ALTER TABLE captured_prompts RENAME TO prompt_captured;
ALTER INDEX captured_prompts_pkey       RENAME TO prompt_captured_pkey;   -- renombra tambien el constraint pkey
ALTER INDEX captured_prompts_status_idx RENAME TO prompt_captured_status_idx;
ALTER INDEX captured_prompts_tsv_idx    RENAME TO prompt_captured_tsv_idx;   -- GIN, conserva metodo
ALTER TABLE prompt_captured RENAME CONSTRAINT captured_prompts_project_id_fkey TO prompt_captured_project_id_fkey;
ALTER TABLE prompt_captured RENAME CONSTRAINT captured_prompts_session_id_fkey TO prompt_captured_session_id_fkey;
ALTER TABLE prompt_captured RENAME CONSTRAINT captured_prompts_user_id_fkey    TO prompt_captured_user_id_fkey;
-- NOTA: NO se renombra el pkey via RENAME CONSTRAINT (ya lo hizo el ALTER INDEX → duplicado).

-- 2) clients → project_clients (2 FK entrantes preservadas por OID)
ALTER TABLE clients RENAME TO project_clients;
ALTER INDEX clients_pkey       RENAME TO project_clients_pkey;
ALTER INDEX clients_status_idx RENAME TO project_clients_status_idx;
ALTER TABLE project_clients RENAME CONSTRAINT clients_status_check TO project_clients_status_check;

-- 3) imported_workflow_files → project_imported_workflow_files
ALTER TABLE imported_workflow_files RENAME TO project_imported_workflow_files;
ALTER INDEX imported_workflow_files_pkey        RENAME TO project_imported_workflow_files_pkey;
ALTER INDEX imported_workflow_files_project_idx RENAME TO project_imported_workflow_files_project_idx;
ALTER INDEX imported_workflow_files_status_idx  RENAME TO project_imported_workflow_files_status_idx;
ALTER INDEX imported_workflow_files_tool_idx    RENAME TO project_imported_workflow_files_tool_idx;
ALTER INDEX imported_workflow_files_unique      RENAME TO project_imported_workflow_files_unique;  -- UNIQUE: solo ALTER INDEX
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_project_id_fkey   TO project_imported_workflow_files_project_id_fkey;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_source_tool_check TO project_imported_workflow_files_source_tool_check;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_status_check      TO project_imported_workflow_files_status_check;

-- 4) observations → knowledge_observations (ivfflat + GIN conservan metodo)
ALTER TABLE observations RENAME TO knowledge_observations;
ALTER INDEX observations_pkey                RENAME TO knowledge_observations_pkey;
ALTER INDEX observations_status_idx          RENAME TO knowledge_observations_status_idx;
ALTER INDEX observations_content_tsv_idx     RENAME TO knowledge_observations_content_tsv_idx;
ALTER INDEX observations_dedup_hash_uniq     RENAME TO knowledge_observations_dedup_hash_uniq;   -- parcial, conserva WHERE
ALTER INDEX observations_embedding_idx       RENAME TO knowledge_observations_embedding_idx;     -- ivfflat, conserva metodo
ALTER INDEX observations_project_created_idx RENAME TO knowledge_observations_project_created_idx;
ALTER INDEX observations_tags_idx            RENAME TO knowledge_observations_tags_idx;
ALTER TABLE knowledge_observations RENAME CONSTRAINT observations_created_by_fkey TO knowledge_observations_created_by_fkey;
ALTER TABLE knowledge_observations RENAME CONSTRAINT observations_project_id_fkey TO knowledge_observations_project_id_fkey;

-- 5+6) par webhook outbound (FK interna, juntos)
ALTER TABLE outbound_webhook_subscriptions RENAME TO webhook_outbound_subscriptions;
ALTER INDEX outbound_webhook_subscriptions_pkey       RENAME TO webhook_outbound_subscriptions_pkey;
ALTER INDEX outbound_webhook_subscriptions_status_idx RENAME TO webhook_outbound_subscriptions_status_idx;
ALTER INDEX outbound_webhook_subscriptions_events_gin RENAME TO webhook_outbound_subscriptions_events_gin;
ALTER TABLE webhook_outbound_subscriptions RENAME CONSTRAINT outbound_webhook_subscriptions_url_check TO webhook_outbound_subscriptions_url_check;

ALTER TABLE outbound_webhook_deliveries RENAME TO webhook_outbound_deliveries;
ALTER INDEX outbound_webhook_deliveries_pkey        RENAME TO webhook_outbound_deliveries_pkey;
ALTER INDEX outbound_webhook_deliveries_status_idx  RENAME TO webhook_outbound_deliveries_status_idx;
ALTER INDEX outbound_webhook_deliveries_pending_idx RENAME TO webhook_outbound_deliveries_pending_idx;
ALTER INDEX outbound_webhook_deliveries_sub_idx     RENAME TO webhook_outbound_deliveries_sub_idx;
ALTER TABLE webhook_outbound_deliveries RENAME CONSTRAINT outbound_webhook_deliveries_status_check         TO webhook_outbound_deliveries_status_check;
ALTER TABLE webhook_outbound_deliveries RENAME CONSTRAINT outbound_webhook_deliveries_subscription_id_fkey TO webhook_outbound_deliveries_subscription_id_fkey;

-- 7+8) par runner self-hosted (FK interna, juntos)
ALTER TABLE selfhosted_runners RENAME TO runner_selfhosted;
ALTER INDEX selfhosted_runners_pkey          RENAME TO runner_selfhosted_pkey;
ALTER INDEX selfhosted_runners_status_idx    RENAME TO runner_selfhosted_status_idx;
ALTER INDEX selfhosted_runners_heartbeat_idx RENAME TO runner_selfhosted_heartbeat_idx;

ALTER TABLE selfhosted_tasks RENAME TO runner_selfhosted_tasks;
ALTER INDEX selfhosted_tasks_pkey        RENAME TO runner_selfhosted_tasks_pkey;
ALTER INDEX selfhosted_tasks_status_idx  RENAME TO runner_selfhosted_tasks_status_idx;
ALTER INDEX selfhosted_tasks_claimed_idx RENAME TO runner_selfhosted_tasks_claimed_idx;
ALTER TABLE runner_selfhosted_tasks RENAME CONSTRAINT selfhosted_tasks_status_check     TO runner_selfhosted_tasks_status_check;
ALTER TABLE runner_selfhosted_tasks RENAME CONSTRAINT selfhosted_tasks_claimed_by_fkey  TO runner_selfhosted_tasks_claimed_by_fkey;

-- 9) activity_log → audit_activity_log (FORCE RLS inerte, sin policy)
ALTER TABLE activity_log RENAME TO audit_activity_log;
ALTER INDEX activity_log_pkey        RENAME TO audit_activity_log_pkey;
ALTER INDEX activity_log_status_idx  RENAME TO audit_activity_log_status_idx;
ALTER INDEX activity_log_actor_idx   RENAME TO audit_activity_log_actor_idx;
ALTER INDEX activity_log_entity_idx  RENAME TO audit_activity_log_entity_idx;
ALTER INDEX activity_log_project_idx RENAME TO audit_activity_log_project_idx;
ALTER TABLE audit_activity_log RENAME CONSTRAINT activity_log_actor_id_fkey    TO audit_activity_log_actor_id_fkey;
ALTER TABLE audit_activity_log RENAME CONSTRAINT activity_log_project_id_fkey  TO audit_activity_log_project_id_fkey;
ALTER TABLE audit_activity_log RENAME CONSTRAINT activity_log_visibility_check TO audit_activity_log_visibility_check;

COMMIT;
```

El `down` es el reverso exacto, también en una sola transacción (ver archivo `.down.sql`).

## Compatibilidad con squawk

`ALTER TABLE ... RENAME` y `ALTER INDEX ... RENAME` son operaciones de metadata-only: toman un `ACCESS EXCLUSIVE LOCK` pero no reescriben datos ni bloquean por duración significativa. Squawk no marca renames como riesgosos (no son `ADD COLUMN ... DEFAULT` volátil, ni `SET NOT NULL` sin check, ni índice no-concurrente sobre tabla con datos). Las tablas tienen 0 rows: el lock es instantáneo. **No se requiere `CREATE INDEX CONCURRENTLY`** porque no se crean índices, solo se renombran.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Emitir `RENAME CONSTRAINT` sobre pkey además del `ALTER INDEX` → error por duplicado | Para pkey y UNIQUE con índice homónimo se emite SOLO `ALTER INDEX`. Verificado en el DDL arriba (prompt_captured pkey, imported_workflow_files_unique) |
| FK entrantes a `clients` (projects, project_tickets) se rompen | NO se rompen: Postgres resuelve FK por OID. El `ALTER TABLE RENAME` actualiza la metadata automáticamente. Solo el código Go que hace `JOIN clients` debe actualizar el identificador |
| Pares con FK interna en estado intermedio | Renombrar ambos miembros del par dentro de la MISMA tx (webhook subs+deliveries, runner runners+tasks) |
| Dependencia con DROP de `sessions` (FK desde captured_prompts) | El rename NO toca `sessions`. Coordinar orden: si el drop corre antes, ese agente limpia `session_id` (`ON DELETE SET NULL`). Documentado en open_questions del issue |
| `observations` alto fan-out (12 archivos Go) → query olvidada | grep exhaustivo del identificador `observations` (word-boundary) antes de cerrar; test de sabotaje deja una query sin renombrar y verifica que falla |
| Índice ivfflat/GIN pierde método al renombrar | NO ocurre: `ALTER INDEX RENAME` solo cambia el nombre, no la definición. Test verifica `indexdef` post-rename |
| Falso positivo en seeder de policies (`platform_policies_seeder.go` menciona activity_log) | Confirmado: es texto documental, NO identificador SQL. NO se toca |
| Doble rename con HU 42.5/42.7 (enrollment, verifications, verification_results) | DE-SCOPEADAS de esta HU. Confirmar que 000151 (42.5) y 000153 (42.7) las cubren antes de aplicar 000155 |
| Colisión de numeración: 42.5=000151, 42.6=000152, 42.7=000153 ya tomados | Esta HU usa 000155 (deja 000154 de margen). Verificar antes de aplicar que 000155 sigue libre |

## TDD plan

1. **Red:** test de migración que aplica `000155 up` sobre un schema con las 9 tablas originales y verifica `to_regclass('public.<nuevo_nombre>') IS NOT NULL` y `to_regclass('public.<viejo_nombre>') IS NULL` para las 9.
2. **Green:** escribir la migration up con los renames de la sección DDL.
3. **Red:** test que aplica `up` y luego `down`, verificando que el schema vuelve EXACTAMENTE al estado original (nombres de tabla, índices, constraints).
4. **Green:** escribir el `down` como reverso exacto.
5. **Red (índices con método):** test que verifica que `knowledge_observations_embedding_idx` sigue siendo `ivfflat` y `prompt_captured_tsv_idx`/`knowledge_observations_content_tsv_idx` siguen `gin` tras el rename.
6. **Green:** confirmado por la naturaleza de `ALTER INDEX RENAME` (no toca definición).
7. **Sabotaje:** ver `tasks.md` sección Sabotaje — dejar una query Go contra `observations` sin renombrar y confirmar que el test de integración FALLA; y emitir un `RENAME CONSTRAINT` redundante sobre el pkey y confirmar que la migration FALLA por duplicado.

## Estado de la migration

- **Si hay renames reales (caso por defecto):** se emite `000155_rename_resto_taxonomy.up.sql` + `.down.sql` (escritas en esta HU).
- **Si otra HU absorbió todos los renames (no-op):** NO se emite migration. Esta sección se actualiza a: *"No requiere migration: el resto de tablas ya cumple prefijo (keep_ok). HU cerrada como verificación documental contra el último scenario de Gherkin."*
