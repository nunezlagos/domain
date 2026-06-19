# issue-42.9-rename-resto

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** media
**Tipo:** refactor (rename de schema)

## Historia de usuario

**Como** arquitecto del schema de `domain`
**Quiero** renombrar el RESTO de las tablas con `action=rename` que NO entraron en las HU 42.5–42.8, aplicando el prefijo de su grupo funcional según la taxonomía
**Para** que TODA tabla quede agrupable por prefijo (`prompt_`, `project_`, `knowledge_`, `webhook_`, `runner_`, `audit_`) y el admin `/database` muestre las tablas ordenadas por dominio, sin restos de naming legacy

## Alcance de esta HU (tablas cubiertas)

Esta HU es el **cierre de renames** de REQ-42: barre las tablas `action=rename` del `renMisc` que NO fueron absorbidas por las HU 42.5–42.8. **Verificado contra las HU hermanas ya escritas**, tres tablas del `renMisc` original quedan FUERA de esta HU porque ya tienen dueño:

| Tabla | Destino | Dueño | Migration |
|---|---|---|---|
| `org_enrollment_tokens` | `enrollment_tokens` | HU 42.7 | 000153 |
| `verifications` | `tdd_verifications` | HU 42.5 | 000151 |
| `verification_results` | `tdd_verification_results` | HU 42.5 | 000151 |

Por tanto esta HU 42.9 cubre las **9 tablas restantes** (mapeo verificado contra taxonomía + `renMisc`):

| # | Tabla actual | Tabla destino | Grupo | sense_in_flow |
|---|---|---|---|---|
| 1 | `captured_prompts` | `prompt_captured` | prompt | dormant |
| 2 | `clients` | `project_clients` | project | core |
| 3 | `imported_workflow_files` | `project_imported_workflow_files` | project | dormant |
| 4 | `observations` | `knowledge_observations` | knowledge | core |
| 5 | `outbound_webhook_subscriptions` | `webhook_outbound_subscriptions` | webhook | dormant |
| 6 | `outbound_webhook_deliveries` | `webhook_outbound_deliveries` | webhook | dormant |
| 7 | `selfhosted_runners` | `runner_selfhosted` | runner | dormant |
| 8 | `selfhosted_tasks` | `runner_selfhosted_tasks` | runner | dormant |
| 9 | `activity_log` | `audit_activity_log` | audit | dormant |

> **Nota de coordinación:** el `renMisc` que alimentó esta HU traía `enrollment_tokens` como "lead item", pero la HU 42.7 ya lo reclama (migration 000153) y la HU 42.5 ya reclama el par `verifications`/`verification_results` (migration 000151). Para evitar doble rename de la misma tabla, esta HU los DE-SCOPEA explícitamente. Si por algún motivo 42.5/42.7 NO se aplican, habría que reincorporarlos aquí.

**Tablas que NO se tocan en esta HU (keep_ok / cubiertas en otra parte):** todas las `action=keep_ok` de la taxonomía (ya cumplen prefijo), las `action=drop` (HU 42.2/42.3), las SDD/issue/gherkin/auth/users que pertenecen a 42.5–42.8, y las 3 de-scopeadas arriba. Si al ejecutar esta HU NO quedaran renames reales (porque otra HU los absorbió también), esta HU se degrada a **no-op de verificación**: el Gherkin de verificación abajo igual aplica y la migration NO se emite (ver `design.md`).

## Criterios de aceptación

```gherkin
Feature: Rename del resto de tablas a su prefijo de grupo (cierre REQ-42)

  Background:
    Given la base de datos "domain" en single-org
    And la migration previa aplicada es la 000146 (org_flow_config → flow_config)
    And las HU 42.5 (000151) y 42.7 (000153) ya reclaman enrollment_tokens/verifications/verification_results
    And ninguna de las 9 tablas objetivo de esta HU tiene RLS policy activa (relrowsecurity=f)
    And ninguna tiene sequence propia (todas con PK UUID)

  Scenario: Rename de captured_prompts a prompt_captured con pkey sin duplicar
    Given la tabla "captured_prompts" existe con su pkey y 2 índices extra
    When se aplica la migration 000155 up
    Then existe la tabla "prompt_captured"
    And NO existe la tabla "captured_prompts"
    And el índice "prompt_captured_pkey" existe (renombrado, comparte objeto con el constraint pkey)
    And el índice GIN "prompt_captured_tsv_idx" conserva su método (USING gin)
    And la FK "prompt_captured_session_id_fkey" sigue apuntando a la tabla de sesiones por OID
    And NO se emitió un ALTER TABLE RENAME CONSTRAINT sobre el pkey (lo renombra el ALTER INDEX)

  Scenario: Rename de clients preserva las FK entrantes por OID
    Given la tabla "clients" tiene 2 FK entrantes (projects.client_id, project_tickets.client_id)
    When se aplica la migration 000155 up
    Then existe la tabla "project_clients"
    And projects.projects_client_id_fkey sigue resolviendo contra project_clients (por OID, sin cambio de nombre)
    And project_tickets.project_tickets_client_id_fkey sigue válida

  Scenario: Rename de imported_workflow_files con constraint UNIQUE de índice homónimo
    Given la tabla "imported_workflow_files" tiene la constraint UNIQUE "imported_workflow_files_unique" con índice homónimo
    When se aplica la migration 000155 up
    Then existe la tabla "project_imported_workflow_files"
    And el índice/constraint "project_imported_workflow_files_unique" existe UNA sola vez
    And NO se emitió a la vez ALTER INDEX RENAME y ALTER TABLE RENAME CONSTRAINT sobre el mismo nombre

  Scenario: El par TDD verifications + verification_results NO se toca aquí
    Given las tablas "verifications" y "verification_results" son responsabilidad de la HU 42.5 (000151)
    When se aplica la migration 000155 up
    Then la migration 000155 NO contiene ningún ALTER sobre verifications ni verification_results
    And el rename de esas tablas queda delegado a la migration 000151

  Scenario: Rename de observations (alto fan-out) a knowledge_observations
    Given la tabla "observations" tiene un índice ivfflat y un índice GIN
    When se aplica la migration 000155 up
    Then existe la tabla "knowledge_observations"
    And el índice "knowledge_observations_embedding_idx" conserva USING ivfflat (vector_cosine_ops)
    And el índice parcial "knowledge_observations_dedup_hash_uniq" conserva su predicado WHERE

  Scenario: Rename del par webhook outbound en la misma transacción
    Given "outbound_webhook_subscriptions" y "outbound_webhook_deliveries" existen
    And deliveries tiene FK subscription_id hacia subscriptions
    When se aplica la migration 000155 up
    Then existen "webhook_outbound_subscriptions" y "webhook_outbound_deliveries"
    And la FK "webhook_outbound_deliveries_subscription_id_fkey" sigue válida (preservada por OID)

  Scenario: Rename del par runner self-hosted en la misma transacción
    Given "selfhosted_runners" y "selfhosted_tasks" existen
    And tasks tiene FK claimed_by hacia runners
    When se aplica la migration 000155 up
    Then existen "runner_selfhosted" y "runner_selfhosted_tasks"
    And la FK "runner_selfhosted_tasks_claimed_by_fkey" sigue válida (preservada por OID)

  Scenario: Rename de activity_log a audit_activity_log (FORCE RLS inerte)
    Given "activity_log" tiene relforcerowsecurity=t pero relrowsecurity=f (FORCE sin ENABLE, inerte)
    And NO existe ninguna policy sobre activity_log
    When se aplica la migration 000155 up
    Then existe la tabla "audit_activity_log"
    And NO se intentó renombrar ni recrear ninguna policy

  Scenario: Rollback completo (down)
    Given la migration 000155 fue aplicada
    When se aplica la migration 000155 down
    Then las 9 tablas vuelven a sus nombres originales
    And todos los índices y constraints vuelven a su nombre original
    And la migration 000155 down NO toca enrollment_tokens ni el par tdd_verification*

  Scenario: Verificación no-op (si otra HU absorbió los renames)
    Given todas las tablas objetivo YA tienen su prefijo de grupo
    When reviso el schema actual
    Then no quedan tablas con action=rename pendientes en este alcance
    And esta HU se documenta como cumplida (keep_ok) sin migration nueva
```

## Análisis breve

- **Qué pide realmente:** cerrar el rename de schema de REQ-42 aplicando prefijo de grupo a las 9 tablas restantes (las del `renMisc` no absorbidas por 42.5–42.8). Es un rename atómico estilo 000146, sin expand/contract (single-org, DB casi vacía, riesgo de datos NULO).
- **Por qué es seguro:** ninguna de estas tablas tiene sequence (todas PK UUID), ninguna tiene RLS policy activa (las únicas con policy son `audit_log` y `otp_codes`, fuera de este alcance), y las FK se preservan por OID en Postgres (el `ALTER TABLE RENAME` no rompe referencias entrantes ni salientes).
- **Trampas detectadas (verificadas contra la introspección):**
  1. **pkey duplicado:** en Postgres el pkey existe como índice Y como constraint pero comparten el mismo objeto. `ALTER INDEX <pkey> RENAME` renombra ambos. Emitir TAMBIÉN `ALTER TABLE RENAME CONSTRAINT` sobre el pkey FALLA por duplicado. Lo mismo para las constraints UNIQUE con índice homónimo (`imported_workflow_files_unique`).
  2. **Pares con FK interna:** webhook (subscriptions↔deliveries) y runner (runners↔tasks) deben renombrarse en la MISMA transacción.
  3. **Dependencia con DROP de `sessions`:** `captured_prompts.session_id` tiene FK a `sessions` (marcada DROP por otro agente, `ON DELETE SET NULL`). El rename es independiente del drop, pero hay que **coordinar el orden de migraciones**: si el drop de `sessions` corre antes, ese agente limpia la columna `session_id`. Esta HU NO toca `sessions`.
  4. **`observations` es alto fan-out:** 12 archivos Go no-test referencian la tabla. El rename del identificador SQL hay que hacerlo query por query.
  5. **De-scope de colisiones:** `enrollment_tokens` (HU 42.7), `verifications` y `verification_results` (HU 42.5) NO se renombran aquí para evitar doble rename. La normalización del índice `verification_task_id_idx → tdd_verification_results_task_id_idx` es responsabilidad de la HU 42.5.
- **Módulos a tocar (código Go, fuera de la migration):** ver `tasks.md` sección "Touchpoints de código". El rename de tabla en SQL exige actualizar cada `FROM`/`JOIN`/`INSERT INTO` en los repositorios, herramientas MCP, handlers REST, health checks y el seeder demo.
- **Esfuerzo tentativo:** M (la migration es mecánica; el fan-out de código en `observations` y el health check multi-tabla suben el costo).

## Open questions

1. **De-scope confirmado:** `enrollment_tokens` lo hace la HU 42.7 (000153) y el par `verifications`/`verification_results` la HU 42.5 (000151). Esta HU los excluye. ¿Confirmás que 42.5 y 42.7 efectivamente se aplican (sino habría que reincorporarlos aquí)?
2. **`captured_prompts` → `prompt_captured` vs keep_ok:** solo es duda de naming. ¿Confirmás el rename?
3. **`clients` → `project_clients` y `observations` → `knowledge_observations`:** ¿OK agruparlas bajo `project_`/`knowledge_` o preferís grupos propios (`client_`, `memory_`)?
4. **`outbound_webhook_*` → `webhook_outbound_*`:** ¿OK reordenar el prefijo o mantener (keep_ok)?
5. **`selfhosted_*` → `runner_*`:** ¿OK reagrupar bajo `runner_` o mantener grupo `selfhosted_`?
6. **`activity_log` → `audit_activity_log`:** ¿audit_ o grupo `activity_` propio?
7. **Orden vs DROP de `sessions`:** confirmar qué migration corre primero (el drop de `sessions` o este rename) para coordinar la limpieza de `session_id` en `captured_prompts`.

## Verificación previa

- [ ] Confirmar que 000155 está libre y NO colisiona con 000151 (42.5), 000152 (42.6), 000153 (42.7)
- [ ] Confirmar que las HU 42.5/42.7 efectivamente cubren enrollment_tokens/verifications/verification_results (de-scope válido)
- [ ] Confirmar contra la introspección que las 9 tablas NO tienen sequence (PK UUID)
- [ ] Confirmar que NINGUNA de las 9 tiene RLS policy activa (`relrowsecurity=f`; FORCE sin ENABLE es inerte)
- [ ] Confirmar el orden de ejecución respecto al DROP de `sessions` (coordinación de `session_id` en captured_prompts)
- [ ] Listar TODOS los touchpoints de código por tabla (grep de identificador SQL) y separar falsos positivos (texto documental) de identificadores reales

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
