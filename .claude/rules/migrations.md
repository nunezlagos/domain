# Migrations Conventions — Domain

Sistema: `golang-migrate` (issue-01.1). Safety enforced por `squawk` linter (issue-25.3) + conventions enforcement (issue-25.13).

## Naming

```
migrations/
  000001_create_extensions.up.sql
  000001_create_extensions.down.sql
  000002_create_organizations.up.sql
  000002_create_organizations.down.sql
  ...
```

- Numeración secuencial 6-digit zero-padded (`000001`, `000999`)
- Descripción `snake_case` corta
- Par up + down obligatorio
- NUNCA renumerar ni reordenar migrations aplicadas

## Header obligatorio

Cada `.up.sql` empieza con:

```sql
-- migration: add_user_rut_column
-- author: <git_username>
-- issue: #1234 (or issue-XX.Y)
-- description: agrega columna RUT con validation módulo 11
-- breaking: false
-- estimated_duration: 1s (empty table)

ALTER TABLE users ADD COLUMN rut VARCHAR(12) UNIQUE;
```

Campos:
- `migration`: slug coincidente con filename
- `author`: email o github username
- `issue`: HU reference o GitHub issue
- `description`: 1-line qué hace
- `breaking`: `true` si rompe schema previo
- `estimated_duration`: aproximado (afecta lock-timeout)

Enforced por linter (issue-25.13).

## Idempotencia

- TODO statement usa `IF EXISTS` / `IF NOT EXISTS` donde aplique:
  - `CREATE TABLE IF NOT EXISTS`
  - `CREATE INDEX IF NOT EXISTS`
  - `DROP TABLE IF EXISTS ... CASCADE`
- Migrations corridas 2x deben ser no-op en segunda

## Safety enforcement (issue-25.3 squawk)

Bloqueados en CI:
- `CREATE INDEX` sin `CONCURRENTLY`
- `ALTER TABLE ... ADD COLUMN ... NOT NULL` sin `DEFAULT`
- `DROP TABLE` sin `IF EXISTS`
- `ALTER TABLE ... ADD FOREIGN KEY` sin `NOT VALID` + `VALIDATE` posterior
- `VACUUM FULL`
- `LOCK TABLE` explícito sin override

Override comment cuando justificado:
```sql
-- squawk-ignore: require-concurrent-index-creation
-- reason: empty table, no traffic yet
CREATE INDEX idx_x ON new_table(col);
```

## Conventions enforcement (issue-25.13)

- Naming snake_case plural (`organizations`, no `organization`)
- Columnas estándar: `id UUID`, `created_at`, `updated_at` cuando aplica
- Tipos canónicos (ver `db.md`): UUID/BIGSERIAL, TIMESTAMPTZ, JSONB, NUMERIC para money
- FKs siempre `<singular>_id`
- Trigger `set_updated_at_<table>` cuando tabla tiene `updated_at`

## Down migrations

- Cada `up.sql` requiere `down.sql` correspondiente
- Down hace inverso EXACTO en orden inverso
- Down testeable: `migrate down 1 && migrate up 1` debe ser idempotente
- Excepción: migrations que pueden tener datos (down con `IF EXISTS` y warn de pérdida)

## Migrations grandes

Para tablas con muchas filas o operaciones costosas:

1. **Split en múltiples migrations** (no una grande)
2. **Backfill en batches** con script separado (no en migration), ejemplo:
   ```sql
   -- migration 000XXX: add column nullable
   ALTER TABLE big_table ADD COLUMN new_col TEXT;

   -- separate backfill script (cmd/backfill-XXX/main.go)
   -- corre en background con UPDATE WHERE ... LIMIT 1000

   -- migration 000YYY: set NOT NULL once backfilled
   ALTER TABLE big_table ALTER COLUMN new_col SET NOT NULL;
   ```
3. **Documentar duración esperada** en header
4. **Considerar `SET lock_timeout`** local antes del ALTER

## Patrón "expand/contract" para cambios breaking

Para renombrar columna o cambiar tipo sin downtime:

```
Phase 1 (expand): ADD new column nullable, copy data en backfill, dual-write en código
Phase 2 (migrate): app pasa a leer new column
Phase 3 (contract): DROP old column en migration posterior
```

3+ deploys separados. NO juntar en 1 migration.

## Seed data — NO en migrations

Migrations son **schema only**. Para data inicial (planes, model registry, templates):
- Usar sistema de seeders (issue-01.7)
- Seeders son idempotentes (UPSERT)
- Migrations no contienen INSERT salvo enums de tipo lookup pequeños

Excepción tolerable: filas de configuración esenciales (ej: row default en `api_versions` tabla) si <10 rows y nunca cambian.

## Roll-forward, no roll-back en prod

- Down migrations existen para dev/staging
- En prod: **fix forward con nueva migration**, no `migrate down`
- Si una migration prod rompe: nueva migration que repare, NO down

## Migration dry-run

Antes de aplicar en prod:
1. `make db-lint` local
2. CI corrida en kind cluster
3. Aplicar primero en staging
4. Drill restore después en issue-18.3

## Versioning de schema

- `schema_migrations` tabla manejada por golang-migrate
- `domain version` CLI reporta version del binary + migration version actual
- Hook helm pre-upgrade aplica migration antes de rollout pods (issue-24.1)

## Anti-patterns prohibidos

- ❌ Modificar migration ya aplicada en prod
- ❌ `INSERT INTO foo VALUES ...` (usar seeders)
- ❌ Lógica de negocio en triggers o stored procedures
- ❌ Procedures complejos (mantenibilidad)
- ❌ `CREATE INDEX` no concurrent en tabla con datos
- ❌ `NOT NULL` sin `DEFAULT` o backfill previo
- ❌ Renombrar columna en una sola migration (usar expand/contract)
- ❌ Down que pierde datos sin warning explícito
