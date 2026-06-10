# Design: issue-18.1-postgres-backups

## Decisión arquitectónica

**Herramienta:** pgBackRest 2.x (no pg_basebackup raw).
**Repositorio:** S3 (AWS, R2, MinIO compatible).
**Cron:** externo (Kubernetes CronJob o systemd timer), no en el container postgres.
**Sidecar exporter:** opcional, parsea `pgbackrest info --output=json`.

## Alternativas descartadas

- **pg_basebackup + script bash:** sin retention, sin incremental, sin PITR robusto
- **WAL-G:** competidor válido, pero pgBackRest tiene mejor handling de cluster + monitoring
- **Barman:** push-mode requiere SSH; pgBackRest soporta S3 nativo

## Componentes

```
deploy/postgres/
  Dockerfile.pgvector-backup      → FROM pgvector/pgvector:pg16 + pgbackrest
  pgbackrest.conf                  → stanza domain, repo s3
  archive_command.sh               → pgbackrest --stanza=domain archive-push %p
deploy/cron/
  backup-full.cron                 → "0 2 * * *" → pgbackrest backup --type=full
  backup-incr.cron                 → "0 */4 * * *" → pgbackrest backup --type=incr
  expire.cron                      → "0 3 * * *" → pgbackrest expire
```

## Configuración pgbackrest.conf

```ini
[global]
repo1-type=s3
repo1-s3-bucket=domain-pg-backups
repo1-s3-region=us-east-1
repo1-s3-endpoint=s3.amazonaws.com
repo1-retention-full=4
repo1-cipher-type=aes-256-cbc
process-max=4
compress-type=zst

[domain]
pg1-path=/var/lib/postgresql/data
pg1-port=5432
```

## TDD plan

1. Backup full → restore en cluster nuevo → checksums match
2. WAL push latency <30s con monitor
3. Drill PITR a timestamp T → query devuelve estado en T
4. Cron failure → métrica y alerta
5. Retention: 5to backup full borra el 1ro

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Repo S3 inaccesible | Healthcheck que falla loud + canary backup |
| Cipher key perdida | Key en KMS/Vault, rotación documentada |
| Storage cost | Retention agresiva por plan + compresión zst |
