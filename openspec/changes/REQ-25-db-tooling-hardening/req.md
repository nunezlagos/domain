# REQ-25-db-tooling-hardening: Tooling y hardening de Postgres: PgBouncer, slow query analysis, migration linter, RLS selectivo, least-privilege roles, pgaudit, resource limits, read replicas, secrets rotation, anonymization, monitoring de locks/vacuum.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F1, F4

## Descripción

Capa transversal de operabilidad y seguridad sobre el Postgres de Domain. Cubre el ciclo: conexiones (pooler), observabilidad (slow queries, locks, vacuum), prevención (migration linter, schema drift), seguridad (RLS selectivo, least-privilege, pgaudit), performance (read replicas, resource limits) y operación (secrets rotation, anonymization).

## Criterios de éxito

- Connection pooler PgBouncer (transaction-pooling) delante de Postgres con HA y monitoring
- `pg_stat_statements` + `auto_explain` habilitados con análisis automático y slow query alerts (>100ms p95)
- Migration linter (squawk o atlas) bloquea PRs con migraciones peligrosas en CI
- Schema drift detection cron compara DB real vs migraciones aplicadas y alerta
- RLS policies activas en tablas sensibles (secrets, billing_*, audit_log, sessions, otp_codes, idempotency_records, notification_deliveries, custom_roles)
- Roles least-privilege: `app_user` (CRUD only), `app_migrator` (DDL CI), `app_readonly`, `app_admin`
- `pgaudit` habilitado capturando DDL + ROLE + sensitive operations
- Resource limits configurados: `statement_timeout=30s`, `lock_timeout=10s`, `idle_in_transaction_session_timeout=60s`, `work_mem` calibrado
- Read replica activa desde MVP con routing en pgx (writes → primary, reads pesados → replica)
- DB password rotation sin downtime via dual-credentials window
- Anonymization tooling para dump prod → staging/dev sin PII
- Lock monitoring + autovacuum monitoring + index advisor con alertas

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-25.1-pgbouncer-pooling | proposed | PgBouncer transaction-pooling delante de Postgres, HA, monitoring |
| HU-25.2-pg-stat-statements | proposed | pg_stat_statements + auto_explain + slow query alerts >100ms p95 |
| HU-25.3-migration-linter | proposed | Migration linter (squawk/atlas) en CI bloquea PRs con DDL peligroso |
| HU-25.4-schema-drift | proposed | Cron compara schema real vs migraciones aplicadas + alerta |
| HU-25.5-rls-sensitive-tables | proposed | RLS selectivo en secrets/billing/audit_log/sessions/otp/idempotency |
| HU-25.6-least-privilege-roles | proposed | Roles app_user/app_migrator/app_readonly/app_admin con grants mínimos |
| HU-25.7-pgaudit-db-level | proposed | pgaudit extension capturando DDL + ROLE + sensitive ops paralelo a audit_log app |
| HU-25.8-resource-limits-timeouts | proposed | statement_timeout 30s, lock_timeout 10s, idle_in_tx 60s, work_mem calibrado |
| HU-25.9-read-replicas-routing | proposed | Read replica + router pgx (writes primary, reads pesados replica), lag monitoring |
| HU-25.10-db-secrets-rotation | proposed | DB password rotation sin downtime con dual-credentials window |
| HU-25.11-anonymization-staging | proposed | Tooling dump prod → staging/dev con PII redaction reproducible |
| HU-25.12-locks-vacuum-monitoring | proposed | Lock waits monitoring + autovacuum alerts + index advisor pg_qualstats |
