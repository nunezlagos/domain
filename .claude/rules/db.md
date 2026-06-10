# Database Conventions — Domain

Convenciones obligatorias para schema, migraciones y queries.

## Naming

- **Tablas**: `snake_case`, plural (`organizations`, `agent_runs`, `flow_run_steps`)
- **Columnas**: `snake_case` singular (`organization_id`, `created_at`)
- **FKs**: `<singular>_id` (`organization_id`, no `org_id` ni `org`)
- **Índices**: `<table>_<columns>_idx` para regular, `<table>_<columns>_partial_idx` con WHERE
- **Constraints**: `<table>_<columns>_check`, `<table>_<columns>_unique`
- **Vistas**: sufijo `_view` solo si es vista materializada (`searchable_entities_view`)
- **Tipos enum**: NO usar enums Postgres (rigid); usar `VARCHAR + CHECK` o tabla lookup

## Tipos

| caso | tipo | razón |
|------|------|-------|
| PK de entidades de dominio | `UUID DEFAULT gen_random_uuid()` | distribuible, no leak de cardinalidad |
| PK de tablas de log/append-only | `BIGSERIAL` | volumen alto, secuencial OK |
| Timestamps | `TIMESTAMPTZ DEFAULT NOW()` | NUNCA `TIMESTAMP` sin tz |
| Booleanos | `BOOLEAN` | no `INT`, no `CHAR(1)` |
| Texto corto fijo | `VARCHAR(N)` con CHECK | emails, slugs, country codes |
| Texto libre | `TEXT` | content, body, summary |
| JSON | `JSONB` siempre | nunca `JSON` plain |
| Money | `NUMERIC(12,4)` | nunca `FLOAT` |
| Hashes binarios | `BYTEA` | hash_password, content_hash |
| IPv4/IPv6 | `INET` o `VARCHAR(45)` | preferir `INET` si solo IP |
| Embeddings | `vector(N)` (pgvector) | dimension explícita |
| Arrays | `TEXT[]` solo si <50 elementos esperados | si más, tabla relacional |
| Status enums | `VARCHAR(30)` + CHECK constraint | `status IN ('pending','active','...')` |

## Columnas obligatorias por tabla

Toda tabla de entidad debe tener:
```sql
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
```

Si es entidad scoped por org:
```sql
organization_id UUID NOT NULL REFERENCES organizations(id),
```

Si aplica soft-delete (la mayoría — ver issue-23.2):
```sql
deleted_at TIMESTAMPTZ,
```

## Trigger `updated_at`

```sql
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END
$$ LANGUAGE plpgsql;

-- por cada tabla:
CREATE TRIGGER set_updated_at_<table>
  BEFORE UPDATE ON <table>
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

## Foreign Keys — ON DELETE strategy

| relación | estrategia |
|----------|-----------|
| parent-child fuerte (project → observations) | `ON DELETE CASCADE` |
| ref opcional (flow_run → triggered_by_user) | `ON DELETE SET NULL` |
| ref obligatoria que NO debe perderse (audit_log → actor) | sin FK (append-only) o `ON DELETE RESTRICT` |
| ref cross-org (NUNCA) | bloquear a nivel app + check |

## Índices

- **Default deny**: solo crear índice si una query lo necesita probadamente
- **Partial WHERE** cuando aplique:
  ```sql
  CREATE INDEX ON observations (project_id, created_at)
    WHERE deleted_at IS NULL;
  ```
- **GIN** para `tsvector` y `JSONB` con búsquedas
- **ivfflat** para embeddings (`vector_cosine_ops`)
- **CONCURRENTLY** SIEMPRE en migraciones (issue-25.3 linter lo enforce)

## JSONB

- `JSONB`, NUNCA `JSON`
- Defaults: `DEFAULT '{}'` para objects, `DEFAULT '[]'` para arrays
- Shape documentado en comment SQL o en `/* @schema ... */` (consumido por linter)
- Validar shape con CHECK o trigger si crítico

## Particionado

Particionar por RANGE cuando se cumple:
- Volumen >10M filas/año
- Query patterns con filtro temporal claro
- Necesidad de retention/drop partition

Tablas particionadas existentes:
- `audit_log` (mensual)
- `activity_log` (mensual)
- `cost_logs` (mensual)
- `usage_counters` (mensual)
- `auth_rate_limits` (horaria, drop después 7d)

## Queries — pgx

- SIEMPRE usar parameterized queries (`$1`, `$2`); NUNCA `fmt.Sprintf` con valores
- Usar `pgx.CollectRows`/`CollectOneRow` con `pgx.RowToStructByName`
- TX para multi-write con `pool.BeginTxFunc`
- RLS tables → SIEMPRE via `db.WithOrgTx` helper (issue-25.5)

## Anti-patterns prohibidos

- ❌ `SELECT *` en código (listar columnas)
- ❌ `OFFSET` >10000 (usar cursor pagination)
- ❌ Eager loading N+1 (batch via IN o JOIN)
- ❌ Indices sin propósito (cada índice cuesta en writes)
- ❌ Triggers que disparan lógica de negocio (mantener en service layer)
- ❌ Stored procedures con lógica de negocio (idem)
- ❌ `BETWEEN` con timestamps (off-by-one); usar `>= AND <`
- ❌ Strings concatenados para LIKE: usar `pg_trgm` o FTS

## Encoding y collation

- DB cluster: `ENCODING UTF8`, `LC_COLLATE C.UTF-8`, `LC_CTYPE C.UTF-8`
- Para tsvector con español: `to_tsvector('spanish', col)`
- Slugs en lowercase ASCII strict (validar en app)
