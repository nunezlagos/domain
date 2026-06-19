# Design: issue-42.6-rename-issues

## Decisión arquitectónica

**Rename atómico directo en una sola transacción `BEGIN/COMMIT`**, siguiendo el precedente verificado de la migración `000146_rename_org_flow_config` (single-org, RENAME directo, NO expand/contract). Como el producto es single-org y la tabla está vacía en el VPS, NO hay ventana de escritura concurrente que justifique un patrón expand/contract: el `ALTER TABLE ... RENAME` toma un `ACCESS EXCLUSIVE` lock instantáneo sobre metadata (no reescribe filas) y resuelve en `<1s`.

**Por qué `issue_` y no `sdd_`/`tdd_`:** `gherkin_scenarios` es el contrato de comportamiento (criterios de aceptación BDD) que cuelga del issue vía `issue_id`. Pertenece al dominio del issue, no a la capa de propuesta/diseño (`sdd_*`) ni a la de verificación (`tdd_*`). La taxonomía global lo ubica en el grupo `issue` junto a `issues`, `tasks`, `code_references`, `intake_payloads`.

**Alineación de nombres legacy:** el rename HU→issue previo renombró la COLUMNA `hu_id → issue_id` pero dejó índices/constraints con el nombre viejo (`gherkin_hu_id_idx`, `gherkin_scenarios_hu_id_fkey`). Esta migración aprovecha el rename para alinear TODOS los objetos a `issue_id` + prefijo `issue_`, dejando el schema coherente.

## DDL (up)

Una sola transacción. Orden: tabla → índices → constraint FK. El PK índice y el PK constraint comparten nombre (`gherkin_scenarios_pkey`); en Postgres el `ALTER INDEX ... RENAME` sobre el índice del PK renombra también la constraint asociada, por lo que NO se emite un `RENAME CONSTRAINT` separado para el pkey (evita el error `constraint already renamed`).

```sql
BEGIN;

ALTER TABLE gherkin_scenarios RENAME TO issue_gherkin_scenarios;

ALTER INDEX gherkin_scenarios_pkey        RENAME TO issue_gherkin_scenarios_pkey;
ALTER INDEX gherkin_hu_id_idx             RENAME TO issue_gherkin_scenarios_issue_id_idx;
ALTER INDEX gherkin_scenarios_status_idx  RENAME TO issue_gherkin_scenarios_status_idx;

ALTER TABLE issue_gherkin_scenarios
  RENAME CONSTRAINT gherkin_scenarios_hu_id_fkey
  TO issue_gherkin_scenarios_issue_id_fkey;

COMMIT;
```

## DDL (down)

Reverso simétrico, mismo orden inverso:

```sql
BEGIN;

ALTER TABLE issue_gherkin_scenarios RENAME TO gherkin_scenarios;

ALTER INDEX issue_gherkin_scenarios_pkey            RENAME TO gherkin_scenarios_pkey;
ALTER INDEX issue_gherkin_scenarios_issue_id_idx    RENAME TO gherkin_hu_id_idx;
ALTER INDEX issue_gherkin_scenarios_status_idx      RENAME TO gherkin_scenarios_status_idx;

ALTER TABLE gherkin_scenarios
  RENAME CONSTRAINT issue_gherkin_scenarios_issue_id_fkey
  TO gherkin_scenarios_hu_id_fkey;

COMMIT;
```

## Lo que NO se toca (y por qué)

| Objeto | Decisión | Razón |
|---|---|---|
| sequence | no existe | PK es UUID `gen_random_uuid()`; no hay `*_id_seq` |
| `trg_set_updated_at` | NO renombrar | trigger genérico sin sufijo de tabla; vive en la tabla, sobrevive al RENAME por OID |
| FK saliente `issue_id → issues(id)` | NO recrear | Postgres mantiene la referencia por OID; el rename de la tabla origen no la rompe |
| RLS | nada | la tabla no tiene policies (confirmado en `::POLICIES::`) |
| GRANT | opcional, omitido | la tabla no aparece con grants específicos por rol en la introspección; el RENAME conserva los ACL existentes por OID |

## Touchpoints de código (SQL embebido)

Único repositorio con literales SQL a la tabla — `internal/service/issue/service.go`:

| Línea | Statement | Cambio |
|---|---|---|
| 363 | `SELECT COALESCE(MAX(position), -1) FROM gherkin_scenarios WHERE issue_id = $1` | → `issue_gherkin_scenarios` |
| 368 | `INSERT INTO gherkin_scenarios (...)` (AddScenario) | → `issue_gherkin_scenarios` |
| 381 | `DELETE FROM gherkin_scenarios WHERE id = $1` | → `issue_gherkin_scenarios` |
| 411 | `SELECT ... FROM gherkin_scenarios WHERE issue_id = $1 ORDER BY position` | → `issue_gherkin_scenarios` |
| 425 | `SELECT ... FROM gherkin_scenarios WHERE issue_id = ANY($1) ...` | → `issue_gherkin_scenarios` |
| 462 | `INSERT INTO gherkin_scenarios (...)` (insertScenariosTx) | → `issue_gherkin_scenarios` |

`tests/e2e/schema_audit_test.go` (línea 47): `expectedTables` lista `"gherkin_scenarios"` → `"issue_gherkin_scenarios"`.

> CUIDADO: `gherkin_scenarios` aparece SOLO como literal SQL dentro de backticks, nunca como identificador Go. El search/replace debe acotarse a los 6 literales SQL listados (filtrar por `FROM`/`INTO`/`DELETE FROM`). No hay variables, tipos ni structs Go con ese nombre.

## TDD plan

1. **Red**: test de migración que aplica `000152 up` y verifica con `to_regclass('issue_gherkin_scenarios') IS NOT NULL` y `to_regclass('gherkin_scenarios') IS NULL`.
2. **Green**: escribir el par up/down.
3. **Red**: test que verifica los 3 índices y la constraint FK renombrados (consulta a `pg_indexes` / `pg_constraint`).
4. **Green**: completar el DDL.
5. **Red**: test que verifica que la FK sigue viva (insert con `issue_id` huérfano → error `23503`).
6. **Green**: confirmado por el RENAME (no requiere código extra).
7. **Refactor**: actualizar los 6 literales SQL en `issue/service.go` + `expectedTables` en el audit test.
8. **Sabotaje** (ver tasks.md): introducir el rename de tabla pero OLVIDAR renombrar `gherkin_hu_id_idx` y confirmar que el test de índices FALLA.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Renombrar el pkey por constraint Y por índice → error de doble rename | Solo `ALTER INDEX ... RENAME` sobre el pkey; el constraint del PK se renombra solo |
| Falsos positivos en search/replace de `gherkin_scenarios` | Acotar a los 6 literales SQL en `issue/service.go`; no es identificador Go |
| Olvidar el índice legacy `gherkin_hu_id_idx` (nombre no obvio) | El test de sabotaje verifica explícitamente los 3 nombres de índice destino |
| Romper la FK hacia `issues` | El RENAME mantiene la FK por OID; test de FK viva lo confirma |
| Down asimétrico (dejar nombres a medio camino) | El down invierte exactamente los mismos 4 RENAME en orden inverso |
| `squawk` marca el `ALTER TABLE RENAME` como riesgoso | RENAME es metadata-only, sin reescritura ni lock prolongado; tabla vacía; documentado en el header `breaking: false` |
