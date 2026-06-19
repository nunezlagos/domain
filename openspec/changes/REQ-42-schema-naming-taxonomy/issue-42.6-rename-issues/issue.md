# issue-42.6-rename-issues

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** alta
**Tipo:** refactor (schema naming)

## Historia de usuario

**Como** mantenedor del schema de `domain-backend`
**Quiero** renombrar la tabla `gherkin_scenarios` a `issue_gherkin_scenarios` (con sus índices y constraints alineados a la convención `issue_*` y a la columna `issue_id`)
**Para** que toda tabla perteneciente al dominio del issue lleve el prefijo `issue_` y se pueda agrupar visualmente en el admin `/database`, eliminando además los nombres legacy de la era HU (`gherkin_hu_id_*`, `*_hu_id_fkey`)

## Contexto y alcance

`gherkin_scenarios` almacena los criterios de aceptación BDD (`feature` / `scenario` / `given` / `when` / `then`) atados al issue vía `issue_id`. Es la **capa de especificación de aceptación (BDD)** del pipeline SDD: pertenece al issue, NO al dominio `sdd_*` ni `tdd_*`. Por eso el prefijo elegido es `issue_`.

La taxonomía (`sdd_tdd_analysis`) concluye que `issue_gherkin_scenarios` YA cumple el rol de criterios de aceptación, por lo que NO se crea ninguna tabla nueva (`sdd_acceptance_criteria` queda descartada).

**Esta HU renombra UNA sola tabla:** `gherkin_scenarios → issue_gherkin_scenarios`. Las demás tablas del dominio issue (`tasks`, `code_references`, `intake_payloads`, `issues`) y las de `sdd_*` / `tdd_*` van en HUs hermanas del mismo lote REQ-42; este archivo NO las toca.

### Estado real del objeto (introspección VPS)

- **PK:** `id UUID` con `gen_random_uuid()` → **NO tiene sequence** (no aparece en `::SEQUENCES::`).
- **Índices reales:** `gherkin_scenarios_pkey`, `gherkin_hu_id_idx` (legacy: sin `scenarios`, con `hu`), `gherkin_scenarios_status_idx`.
- **Constraints reales:** `gherkin_scenarios_pkey`, `gherkin_scenarios_hu_id_fkey` (legacy: la columna ya es `issue_id` desde el rename HU→issue, pero el nombre del fkey quedó con `hu`).
- **Trigger:** `trg_set_updated_at` es genérico (NO lleva sufijo de tabla) → sobrevive al RENAME automáticamente. **NO renombrar.**
- **FK saliente:** `gherkin_scenarios.issue_id → issues(id)`. Postgres mantiene la FK por OID; el rename de la tabla NO la rompe.
- **RLS:** ninguna policy (solo `audit_log` y `otp_codes` tienen RLS). NO hay nada que recrear.
- **Datos:** tabla vacía en el VPS → riesgo de datos NULO.

## Criterios de aceptación

```gherkin
Feature: Rename gherkin_scenarios a issue_gherkin_scenarios

  Background:
    Given el schema tiene la tabla "gherkin_scenarios" con PK "gherkin_scenarios_pkey"
    And tiene los índices "gherkin_hu_id_idx" y "gherkin_scenarios_status_idx"
    And tiene la FK "gherkin_scenarios_hu_id_fkey" sobre la columna "issue_id" hacia "issues"
    And la tabla no tiene RLS ni sequence propia

  Scenario: La migración 000152 up renombra tabla, índices y constraints en una sola tx
    When aplico la migración 000152 up
    Then existe la tabla "issue_gherkin_scenarios"
    And NO existe la tabla "gherkin_scenarios"
    And el índice "gherkin_scenarios_pkey" pasó a "issue_gherkin_scenarios_pkey"
    And el índice "gherkin_hu_id_idx" pasó a "issue_gherkin_scenarios_issue_id_idx"
    And el índice "gherkin_scenarios_status_idx" pasó a "issue_gherkin_scenarios_status_idx"
    And la constraint "gherkin_scenarios_hu_id_fkey" pasó a "issue_gherkin_scenarios_issue_id_fkey"

  Scenario: La FK hacia issues sigue viva tras el rename
    When aplico la migración 000152 up
    Then la FK "issue_gherkin_scenarios_issue_id_fkey" sigue apuntando a "issues(id)"
    And insertar un escenario con un issue_id inexistente falla por violación de FK

  Scenario: El trigger genérico de updated_at sobrevive sin renombrarse
    When aplico la migración 000152 up
    Then el trigger "trg_set_updated_at" sigue activo sobre "issue_gherkin_scenarios"
    And actualizar una fila refresca "updated_at"

  Scenario: La migración 000152 down revierte el rename de forma simétrica
    Given apliqué la migración 000152 up
    When aplico la migración 000152 down
    Then existe la tabla "gherkin_scenarios"
    And el índice "issue_gherkin_scenarios_issue_id_idx" volvió a "gherkin_hu_id_idx"
    And la constraint "issue_gherkin_scenarios_issue_id_fkey" volvió a "gherkin_scenarios_hu_id_fkey"

  Scenario: El repositorio de issues consulta la tabla renombrada
    Given la migración 000152 up está aplicada
    When el service de issue ejecuta AddScenario / RemoveScenario / listScenarios
    Then las queries usan "issue_gherkin_scenarios"
    And ninguna query referencia "gherkin_scenarios"
```

## Mapa de renames (datos verificados)

| Objeto | De | A |
|---|---|---|
| tabla | `gherkin_scenarios` | `issue_gherkin_scenarios` |
| índice PK | `gherkin_scenarios_pkey` | `issue_gherkin_scenarios_pkey` |
| índice | `gherkin_hu_id_idx` | `issue_gherkin_scenarios_issue_id_idx` |
| índice | `gherkin_scenarios_status_idx` | `issue_gherkin_scenarios_status_idx` |
| constraint PK | `gherkin_scenarios_pkey` | `issue_gherkin_scenarios_pkey` |
| constraint FK | `gherkin_scenarios_hu_id_fkey` | `issue_gherkin_scenarios_issue_id_fkey` |
| sequence | — (UUID PK, no hay) | — |
| RLS policy | — (no hay) | — |
| trigger | `trg_set_updated_at` (genérico) | **NO se renombra** |

## Análisis breve

- **Qué pide realmente:** un rename atómico tabla + índices + constraints, alineando los nombres legacy `hu` a `issue_id` y agregando el prefijo `issue_`. Sin tocar datos, FKs entrantes/salientes ni trigger genérico.
- **Módulos a tocar (código):** un único repositorio con SQL embebido (`internal/service/issue/service.go`) y el test de auditoría de schema (`tests/e2e/schema_audit_test.go`).
- **Riesgos / dependencias:** falsos positivos al hacer search/replace de `gherkin_scenarios` (la cadena aparece solo como literal SQL en `issue/service.go`, no como identificador Go). La FK `issue_id → issues` se mantiene por OID; NO depende de otra HU del lote. El precedente de método es la migración 000146 (rename atómico en una sola tx).
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Confirmar contra la introspección que los nombres reales de índices/constraints son `gherkin_scenarios_pkey`, `gherkin_hu_id_idx`, `gherkin_scenarios_status_idx`, `gherkin_scenarios_hu_id_fkey`
- [ ] Confirmar que NO hay sequence asociada (UUID PK con `gen_random_uuid()`)
- [ ] Confirmar que NO hay RLS policy sobre la tabla
- [ ] Confirmar que el trigger es el genérico `trg_set_updated_at` (sin sufijo) y NO debe renombrarse
- [ ] Confirmar que el único archivo Go con SQL a la tabla es `internal/service/issue/service.go` (líneas 363/368/381/411/425/462)
- [ ] Confirmar que `tests/e2e/schema_audit_test.go` lista `gherkin_scenarios` en `expectedTables` (línea 47)
- [ ] Confirmar que la próxima migración libre es 000152 dentro del lote REQ-42

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
