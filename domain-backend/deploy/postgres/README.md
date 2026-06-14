# Postgres Server Configuration

Configuración a nivel de cluster Postgres (no manejado por migrations Go).

## Archivos

- `pgaudit.conf` — HU-25.7 auditoría a nivel DB (DDL, ROLE, WRITE).
- `postgresql.tuned.conf` — (futuro) tuning específico Domain workload.

## Cómo aplicar `pgaudit.conf`

### Docker / docker-compose

Montar como volumen en `/etc/postgresql/conf.d/pgaudit.conf` y asegurarse
que `postgresql.conf` tiene:

```conf
shared_preload_libraries = 'pg_stat_statements,pgaudit'
include_dir = '/etc/postgresql/conf.d'
```

### K8s con CloudNativePG / Zalando postgres-operator

```yaml
postgresql:
  parameters:
    shared_preload_libraries: "pg_stat_statements,pgaudit"
    pgaudit.log: "ddl, role, write"
    pgaudit.log_relation: "on"
    pgaudit.log_catalog: "off"
    pgaudit.log_parameter: "off"
```

### Verificación post-deploy

```sql
SHOW shared_preload_libraries;
-- → pg_stat_statements,pgaudit

CREATE EXTENSION IF NOT EXISTS pgaudit;
SELECT * FROM pg_extension WHERE extname = 'pgaudit';

-- Test: cualquier DDL aparece en logs con prefijo AUDIT:
CREATE TABLE _audit_test (x int);
DROP TABLE _audit_test;
-- En el server log:
-- AUDIT: SESSION,1,1,DDL,CREATE TABLE,TABLE,_audit_test,...
```

## Log shipping

Filebeat/promtail debe filtrar líneas que matcheen regex `AUDIT:` y rutearlas
a un índice/bucket separado con retention 7 años (compliance HU-25.7
escenario 4).

Ejemplo promtail pipeline:

```yaml
- match:
    selector: '{app="postgres"}'
    stages:
      - regex:
          expression: 'AUDIT:'
      - labels:
          audit: "true"
      - output:
          source: message
```

## Performance overhead

pgaudit con `log = 'ddl, role, write'` añade ~5-10% overhead bajo carga
con muchos writes. DDL/ROLE son raros → overhead despreciable.

Si se habilita `read` o `function` el overhead puede llegar a 20%+.
Mejor usar object-level audit (security label en tablas específicas) para
SELECT en tablas sensibles, en lugar de blanket `log = read`.
