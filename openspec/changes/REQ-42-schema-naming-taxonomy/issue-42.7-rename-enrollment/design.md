# Design: issue-42.7-rename-enrollment

## Decisión arquitectónica

**Rename atómico directo (NO expand/contract).** El producto es single-org, la tabla tiene **0 filas reales** y **0 FKs entrantes**, por lo que un rename in-place dentro de una sola transacción `BEGIN/COMMIT` es seguro: no hay datos que migrar ni lectores externos que coordinar. Se replica exactamente el patrón del precedente 000146 (`org_flow_config → flow_config`), con dos diferencias:

1. **Sin `ALTER SEQUENCE`:** la PK es `UUID DEFAULT gen_random_uuid()`, no hay `_id_seq`.
2. **Sin RLS:** la tabla no tiene policy (`relrowsecurity = f`); no hay `ALTER POLICY` que emitir.

El despliegue del DDL y del código Go va en el **mismo PR/commit**, porque el rename de la tabla y las queries Go que la nombran deben moverse juntos (el identificador de tabla está hardcodeado en los SQL strings de Go).

## DDL — migración 000153 (up)

```sql
BEGIN;

-- 1. Tabla
ALTER TABLE org_enrollment_tokens RENAME TO enrollment_tokens;

-- 2. Índices (incluye pkey: ALTER INDEX renombra índice + constraint a la vez)
ALTER INDEX org_enrollment_tokens_pkey                  RENAME TO enrollment_tokens_pkey;
ALTER INDEX org_enrollment_tokens_prefix_idx            RENAME TO enrollment_tokens_prefix_idx;
ALTER INDEX org_enrollment_tokens_singleton_active_uniq RENAME TO enrollment_tokens_singleton_active_uniq;
ALTER INDEX org_enrollment_tokens_status_idx            RENAME TO enrollment_tokens_status_idx;

-- 3. Constraints SIN índice propio (FK + CHECK). El pkey NO se incluye acá:
--    ya quedó renombrado por el ALTER INDEX de arriba (mismo objeto).
ALTER TABLE enrollment_tokens
  RENAME CONSTRAINT org_enrollment_tokens_created_by_user_id_fkey
  TO enrollment_tokens_created_by_user_id_fkey;
ALTER TABLE enrollment_tokens
  RENAME CONSTRAINT org_enrollment_tokens_role_check
  TO enrollment_tokens_role_check;

COMMIT;
```

## DDL — migración 000153 (down)

Reverso exacto, mismo orden lógico invertido, en una sola tx:

```sql
BEGIN;

ALTER TABLE enrollment_tokens RENAME TO org_enrollment_tokens;

ALTER INDEX enrollment_tokens_pkey                  RENAME TO org_enrollment_tokens_pkey;
ALTER INDEX enrollment_tokens_prefix_idx            RENAME TO org_enrollment_tokens_prefix_idx;
ALTER INDEX enrollment_tokens_singleton_active_uniq RENAME TO org_enrollment_tokens_singleton_active_uniq;
ALTER INDEX enrollment_tokens_status_idx            RENAME TO org_enrollment_tokens_status_idx;

ALTER TABLE org_enrollment_tokens
  RENAME CONSTRAINT enrollment_tokens_created_by_user_id_fkey
  TO org_enrollment_tokens_created_by_user_id_fkey;
ALTER TABLE org_enrollment_tokens
  RENAME CONSTRAINT enrollment_tokens_role_check
  TO org_enrollment_tokens_role_check;

COMMIT;
```

## Por qué `ALTER ... RENAME` y no `DROP/CREATE`

`RENAME` toma un lock `ACCESS EXCLUSIVE` pero es una operación de catálogo O(1): no reescribe la tabla ni reconstruye índices. En una tabla de 0 filas el lock es instantáneo. `DROP/CREATE` perdería datos y reconstruiría los índices innecesariamente. Además `RENAME` preserva los OIDs, por lo que cualquier FK (entrante o saliente) y cualquier vista/plan en caché siguen válidos.

## Compatibilidad con squawk

- `ALTER TABLE ... RENAME` y `ALTER INDEX ... RENAME` no disparan reglas de squawk de reescritura ni de locks largos (no hay `ADD COLUMN ... DEFAULT`, ni `SET NOT NULL`, ni cambios de tipo).
- El par up/down lleva el header de metadatos completo exigido por el linter del repo.
- Idempotencia: el rename de tabla NO usa `IF EXISTS` porque la migración corre una sola vez y un rename condicional ocultaría drift del schema. El precedente 000146 tampoco lo usa. (`squawk` no exige `IF EXISTS` en RENAME.)

## Cambios en código Go (mismo commit)

| Archivo | Qué cambia |
|---|---|
| `internal/auth/bootstrap/service.go` | `INSERT INTO org_enrollment_tokens` (línea ~167) → `enrollment_tokens` |
| `internal/service/enrollment/service.go` | 5 statements (UPDATE/INSERT/SELECT, líneas ~100/115/160/177/224) → `enrollment_tokens` |
| `internal/service/enrollment/service_integration_test.go` | Fixtures/asserts (`SELECT COUNT(*) FROM org_enrollment_tokens`, líneas ~64/66 y comentario) → `enrollment_tokens` |

Ninguna firma de función ni tipo Go cambia: el rename es puramente del identificador SQL embebido en strings.

## TDD plan

1. **Red:** test de integración que hace `SELECT COUNT(*) FROM enrollment_tokens` contra una DB con la migración 000153 aplicada → falla porque el código aún dice `org_enrollment_tokens`.
2. **Green:** aplicar la migración + renombrar el identificador en los 3 archivos Go.
3. **Refactor:** verificar con grep que NO queda ninguna referencia a `org_enrollment_tokens` en código Go (excepto en migraciones históricas 000098/000139/000141/000142, que son inmutables).
4. **Sabotaje:** ver `tasks.md` — dejar adrede una query con el nombre viejo y confirmar que el test de integración la detecta (rojo), luego restaurar.

## Riesgos y mitigaciones

| Riesgo | Severidad | Mitigación |
|---|---|---|
| Una query Go queda apuntando a `org_enrollment_tokens` tras el rename | Alta (fallo en runtime: relation does not exist) | Grep exhaustivo en el cierre + test de integración que ejercita bootstrap/Rotate/Revoke/Enroll. |
| Renombrar el pkey dos veces (ALTER INDEX + RENAME CONSTRAINT) | Media (la migración aborta) | Documentado: el pkey va SOLO por `ALTER INDEX`; el bloque de RENAME CONSTRAINT excluye el pkey. |
| Confundir `singleton_active_uniq` con una constraint UNIQUE | Baja | Es un índice suelto (no constraint declarada): se renombra solo vía `ALTER INDEX`. |
| Otra HU del REQ-42 colisiona con el número 000153 | Media | Coordinar numeración: 000147–000152 quedan para las otras HUs; esta toma 000153. |
| Pérdida de datos en el rename | Nula | Tabla con 0 filas; `RENAME` no toca datos en ningún caso. |
| Drift: down no restaura un estado idéntico | Baja | El down es el reverso 1:1 del up; misma cantidad y tipo de objetos. |

## Notas de naming (para la HU de cierre del REQ-42)

`enrollment_tokens` queda **sin** prefijo de grupo, en tensión con la regla "toda tabla lleva prefijo". La alternativa `auth_enrollment_tokens` agruparía la tabla con el resto del dominio de autenticación. Esta HU honra el rename pedido literal; la agrupación bajo `auth_` se deja como open question del REQ-42.
