# REQ-18-backup-dr: Backups y Disaster Recovery: backups automáticos de Postgres (PITR), replicación S3, runbook de restore probado.

**Estado:** activo
**Creado:** 2026-06-06
**Fase:** F4

## Descripción

Garantizar continuidad operacional ante pérdida de datos o corrupción: backups versionados de Postgres con Point-In-Time Recovery, replicación cross-region de buckets S3, runbook ejecutable y probado de restore con RPO/RTO declarados.

## Criterios de éxito

- Backups full diarios + WAL continuo de Postgres con retención configurable (default 30 días)
- Point-In-Time Recovery operacional hasta cualquier instante dentro de la ventana de retención
- Replicación cross-region del bucket S3 principal con versionado activado y lifecycle rules
- Runbook documentado en `docs/runbooks/restore.md` con steps reproducibles, validado en staging mensualmente
- RPO declarado ≤ 5 minutos, RTO ≤ 1 hora para restore completo de DB + S3

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-18.1-postgres-backups | proposed | Backups Postgres con pgBackRest o pg_basebackup + WAL archiving + retención y PITR |
| HU-18.2-s3-replication | proposed | Cross-region replication de S3, versioning, lifecycle policies |
| HU-18.3-restore-runbook | proposed | Runbook ejecutable de restore + drill mensual automatizado en staging |
