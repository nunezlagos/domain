# issue-42.7-rename-enrollment

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** media
**Tipo:** refactor (naming / migración de schema)

## Historia de usuario

**Como** mantenedor del schema de la base de datos
**Quiero** renombrar la tabla `org_enrollment_tokens` a `enrollment_tokens` (sacando el prefijo `org_` legacy del diseño multi-tenant)
**Para** que el nombre refleje el modelo single-org real y deje de inducir a pensar que el token está scopeado por organización

## Criterios de aceptación

```gherkin
Feature: Rename org_enrollment_tokens → enrollment_tokens

  Background:
    Given la migración 000153 está aplicada
    And el producto es single-org (sin columna organization_id activa)

  Scenario: La tabla cambió de nombre conservando los datos
    When consulto el catálogo de Postgres por la tabla "enrollment_tokens"
    Then la tabla existe
    And la tabla "org_enrollment_tokens" ya NO existe
    And el conteo de filas es idéntico al previo al rename

  Scenario: Los índices fueron renombrados sin perder su definición
    When listo los índices de "enrollment_tokens"
    Then existe "enrollment_tokens_pkey" (PRIMARY KEY sobre id)
    And existe "enrollment_tokens_prefix_idx" (parcial WHERE revoked_at IS NULL)
    And existe "enrollment_tokens_singleton_active_uniq" (parcial UNIQUE sobre (TRUE) WHERE revoked_at IS NULL)
    And existe "enrollment_tokens_status_idx"
    And ningún índice conserva el prefijo "org_enrollment_tokens_"

  Scenario: Las constraints fueron renombradas
    When listo las constraints de "enrollment_tokens"
    Then existe la PRIMARY KEY "enrollment_tokens_pkey"
    And existe la FK "enrollment_tokens_created_by_user_id_fkey" hacia users(id)
    And existe el CHECK "enrollment_tokens_role_check"
    And ninguna constraint conserva el prefijo "org_enrollment_tokens_"

  Scenario: La invariante de token único activo sigue vigente
    Given existe un token con revoked_at IS NULL
    When intento insertar un segundo token con revoked_at IS NULL
    Then la inserción es rechazada por "enrollment_tokens_singleton_active_uniq"

  Scenario: El código de enrollment opera contra el nuevo nombre
    When ejecuto el bootstrap del primer usuario
    And ejecuto Rotate/Revoke/Enroll/GetMetadata del servicio de enrollment
    Then todas las queries SQL apuntan a "enrollment_tokens"
    And ningún statement referencia "org_enrollment_tokens"

  Scenario: El rollback restaura el nombre legacy
    When aplico la migración down 000153
    Then la tabla "org_enrollment_tokens" existe nuevamente
    And índices y constraints recuperan su prefijo "org_enrollment_tokens_"
```

## Inventario de objetos a renombrar (verificado contra el schema real)

| Tipo | Nombre actual | Nombre nuevo |
|---|---|---|
| TABLE | `org_enrollment_tokens` | `enrollment_tokens` |
| INDEX (pkey) | `org_enrollment_tokens_pkey` | `enrollment_tokens_pkey` |
| INDEX | `org_enrollment_tokens_prefix_idx` | `enrollment_tokens_prefix_idx` |
| INDEX (unique partial) | `org_enrollment_tokens_singleton_active_uniq` | `enrollment_tokens_singleton_active_uniq` |
| INDEX | `org_enrollment_tokens_status_idx` | `enrollment_tokens_status_idx` |
| CONSTRAINT (FK) | `org_enrollment_tokens_created_by_user_id_fkey` | `enrollment_tokens_created_by_user_id_fkey` |
| CONSTRAINT (CHECK) | `org_enrollment_tokens_role_check` | `enrollment_tokens_role_check` |

**NO hay sequence:** la PK es `UUID DEFAULT gen_random_uuid()`, no hay `_id_seq` que renombrar (a diferencia del precedente 000146 con PK serial).

**NO hay RLS policy:** `relrowsecurity = f` en el catálogo (`relforcerowsecurity` no aplica sin ENABLE). La tabla NO aparece en el set de policies (solo `audit_log` y `otp_codes` tienen policy). No hay nada que renombrar ni recrear.

**FKs entrantes:** ninguna. Ninguna otra tabla referencia a `org_enrollment_tokens`, así que el rename no arrastra cambios en tablas vecinas.

**FK saliente:** `created_by_user_id_fkey → users(id) ON DELETE SET NULL`. El `ALTER TABLE RENAME` no rompe la referencia (Postgres la resuelve por OID, no por nombre); renombro la constraint por consistencia para que el `down` sea un reverso exacto.

## Detalle técnico crítico: pkey = índice Y constraint (mismo objeto)

En Postgres, una PRIMARY KEY se materializa como un índice único **y** una constraint que comparten el mismo objeto/nombre. Por eso:

- Emito **`ALTER INDEX org_enrollment_tokens_pkey RENAME TO enrollment_tokens_pkey`** (esto renombra ambas caras, índice y constraint, de una sola vez).
- **NO** emito además `ALTER TABLE ... RENAME CONSTRAINT org_enrollment_tokens_pkey ...`: sería un duplicado sobre el mismo objeto y la migración fallaría.
- Lo mismo aplicaría a un UNIQUE respaldado por índice; aquí `singleton_active_uniq` es un índice "suelto" (no una constraint UNIQUE declarada), así que se renombra solo vía `ALTER INDEX`.

Las constraints que SÍ van por `ALTER TABLE ... RENAME CONSTRAINT` son las que NO tienen índice propio: la FK (`created_by_user_id_fkey`) y el CHECK (`role_check`).

## Análisis breve

- **Qué pide realmente:** quitar el prefijo `org_` legacy. Es el "lead item" del REQ-42 para el grupo de tokens de enrolamiento.
- **Módulos a tocar:** `internal/auth/bootstrap/service.go`, `internal/service/enrollment/service.go`, `internal/service/enrollment/service_integration_test.go`. Total: 6 referencias SQL al identificador de tabla (1 + 5 + fixtures de test).
- **Riesgos / dependencias:** riesgo de datos NULO (0 filas en la tabla). El único riesgo es dejar una query Go apuntando al nombre viejo → fallo en runtime. Se mitiga con grep exhaustivo en el cierre.
- **Esfuerzo tentativo:** S

## Tensión con la convención (open question)

La regla del REQ-42 dice "TODA tabla lleva prefijo de su funcionalidad". El nombre pedido literal es `enrollment_tokens` **sin** prefijo de grupo. La alternativa coherente con la taxonomía sería `auth_enrollment_tokens` (grupo `auth_`, junto a `auth_sessions`, `auth_events`, `otp_codes`, `api_keys`, `invitations`).

**Decisión de esta HU:** se respeta el rename pedido literal (`enrollment_tokens`). Queda abierta la pregunta de agruparlo bajo `auth_` en una HU posterior. Ver "Verificación previa".

## Verificación previa

- [ ] Confirmar contra el catálogo real los 4 índices y 3 constraints (hecho: coinciden con el introspect).
- [ ] Confirmar que NO hay sequence (PK UUID) — no emitir `ALTER SEQUENCE`.
- [ ] Confirmar que NO hay RLS policy activa sobre la tabla (`relrowsecurity = f`).
- [ ] Confirmar que NO hay FKs entrantes (ninguna tabla referencia a esta).
- [ ] Confirmar que el pkey se renombra SOLO vía `ALTER INDEX` (no duplicar con RENAME CONSTRAINT).
- [ ] Confirmar las 6 referencias Go al identificador `org_enrollment_tokens` (bootstrap + service + test).
- [ ] DECIDIR: ¿se deja `enrollment_tokens` o se agrupa como `auth_enrollment_tokens` en una HU futura?

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
