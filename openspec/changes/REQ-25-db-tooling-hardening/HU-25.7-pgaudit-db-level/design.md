# Design: HU-25.7-pgaudit-db-level

## Config postgresql.conf

```
shared_preload_libraries = 'pg_stat_statements,auto_explain,pgaudit'

pgaudit.log = 'ddl, role, write'
pgaudit.log_relation = on
pgaudit.log_catalog = off
pgaudit.log_parameter = on
pgaudit.log_statement_once = on
pgaudit.role = 'audit_role'   # optional, for object audit
```

## Object audit (selectivo)

```sql
CREATE ROLE audit_role NOLOGIN;
GRANT SELECT, INSERT, UPDATE, DELETE ON secrets, audit_log, subscriptions, custom_roles TO audit_role;
-- pgaudit.role='audit_role' triggers audit for any action que requiera privileges de audit_role
```

## Logshipper

```yaml
# promtail config snippet
- match:
    selector: '{job="postgres"}'
    stages:
      - regex:
          expression: '^AUDIT: '
      - labels:
          audit: "true"
- ... separate sink for audit=true ...
```

## Retention

- Audit logs: 7 años (compliance SOC2/HIPAA típico)
- Storage: Loki cold + S3 archive después de 90 días

## TDD plan

1. pgaudit cargado tras restart
2. CREATE TABLE → AUDIT log line
3. SELECT secrets → audit line (object audit)
4. INSERT subscriptions → audit line
5. CREATE ROLE password → password redacted
6. Shipper route audit=true label correct
7. Performance overhead bench <5%
