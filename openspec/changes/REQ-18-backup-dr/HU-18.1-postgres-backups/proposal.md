# Proposal: HU-18.1-postgres-backups

## Intención

Backups automatizados full + diferencial + WAL archiving continuo de Postgres usando pgBackRest contra repositorio S3, con retención configurable, PITR operacional y notificación de fallos.

## Scope

**Incluye:**
- Instalación y configuración de pgBackRest en el container postgres (custom image FROM `pgvector/pgvector:pg16`)
- Configuración `archive_mode=on`, `archive_command` y `archive_timeout`
- Stanza `domain` apuntando a bucket S3 (o MinIO en dev)
- Cron Kubernetes/systemd para `backup --type=full` diario y `backup --type=incr` cada N horas
- Retention policy basada en cantidad de full backups
- Healthcheck que verifica último backup exitoso < ventana esperada
- Métrica Prometheus `domain_backup_last_success_timestamp` y `domain_backup_failed_total`
- Integración con REQ-20 para notificar fallos

**No incluye:**
- Restore automático (manual, ver HU-18.3 runbook)
- Backups de S3 (HU-18.2)

## Enfoque técnico

1. Imagen custom `domain/postgres:pg16-pgbackrest` con pgBackRest 2.x
2. ConfigMap/secret con credenciales S3 separadas para backup user
3. Cron externo (Kubernetes CronJob o systemd timer) invoca `pgbackrest backup`
4. Sidecar exporter parsea `pgbackrest info --output=json` y expone como Prometheus

## Riesgos

- pgBackRest no en imagen oficial pgvector → mantener fork pequeño
- Bandwidth/storage costoso para DB grandes → retention agresiva en plan free
- Restore lento si full backup pesa GB → documentar RTO esperado

## Testing

- Backup full en DB con datos → restore en cluster vacío → datos presentes
- WAL push verificable en S3 dentro de 30s
- Cron fallido → métrica y notificación
- Drill de PITR a timestamp específico → datos esperados
