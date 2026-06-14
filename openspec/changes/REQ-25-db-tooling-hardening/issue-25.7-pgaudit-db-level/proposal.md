# Proposal: issue-25.7-pgaudit-db-level

## Intención

Habilitar `pgaudit` para capturar DDL + ROLE + write en tablas sensibles directamente a nivel Postgres. Provee evidence chain inmutable independiente del app-level audit_log.

## Scope

**Incluye:**
- shared_preload + CREATE EXTENSION
- Config: log = 'ddl, role, write' baseline
- Object audit en tablas sensibles (secrets, audit_log, subscriptions, custom_roles)
- Log shipper routing (Loki label `audit=true` o CloudWatch log group separado)
- Retention 7 años compliance
- Password redaction confirmed

**No incluye:**
- READ audit en todas (volumen prohibitivo)
- Custom auditing rules per-statement

## Enfoque técnico

1. postgresql.conf with shared_preload_libraries y pgaudit.* params
2. CREATE EXTENSION en migration
3. Filebeat/Promtail config separa `AUDIT:` prefix
4. Storage Loki/CloudWatch con retention long

## Riesgos

- Volumen logs: object audit limitado a tablas críticas
- Performance: medir overhead; <5% target
- Password leak en logs: pgaudit redacta automáticamente; verify

## Testing

- pgaudit cargado
- DDL captured
- SELECT secrets capturado (object audit)
- INSERT subscriptions capturado
- Password redacted en CREATE ROLE
- Shipper route AUDIT logs aparte
