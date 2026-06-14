# Tasks: issue-18.1-postgres-backups

## Infraestructura

- [x] **bk-001**: `deploy/postgres/Dockerfile.pgvector-backup` con pgBackRest 2.x
- [x] **bk-002**: Configurar `archive_mode=on`, `archive_command`, `archive_timeout=30`
- [x] **bk-003**: `pgbackrest.conf` con stanza `domain` y repo S3
- [x] **bk-004**: Cron full diario + incr cada 4h + expire diario
- [x] **bk-005**: Métrica exporter `domain_backup_last_success_timestamp` y `_failed_total`
- [x] **bk-006**: Integrar notificación de fallo con REQ-20

## Tests

- [x] **test-001**: Backup full + restore en cluster nuevo → checksums match
- [x] **test-002**: WAL push latency <30s (smoke con `pg_switch_wal()`)
- [x] **test-003**: PITR drill a timestamp específico
- [x] **test-004**: Retention: 5to full purga el 1ro
- [x] **sabotaje-001**: Romper credenciales S3 → cron falla → métrica + notificación

## Docs

- [x] **docs-001**: `docs/runbooks/postgres-backup.md` con setup S3 IAM, key rotation, monitoring

## Cierre

- [x] Drill mensual programado en staging (issue-18.3)
