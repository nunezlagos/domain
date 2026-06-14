# Proposal: issue-18.3-restore-runbook

## Intención

Documentar runbook ejecutable de restore (Postgres + S3) y automatizar drill mensual en staging para validar RPO/RTO declarados.

## Scope

**Incluye:**
- `docs/runbooks/restore.md` con secciones: pre-flight, restore Postgres PITR, restore S3 versión, post-flight, rollback
- Script `scripts/drill/restore-drill.sh` que ejecuta restore en cluster efímero
- Cron Kubernetes CronJob mensual que invoca el script
- Reporte autogenerado en `docs/runbooks/drills/YYYY-MM.md`
- Métricas `domain_restore_drill_duration_seconds` y `_last_success_timestamp`

**No incluye:**
- DR automático (failover) — manual por decisión humana
- Restore selectivo a nivel tabla (futuro)

## Enfoque técnico

1. Markdown estructurado con TOC y bloques de código copiables
2. Script bash idempotente con steps numerados y logs timestamped
3. Cluster efímero en staging Kubernetes con StatefulSet + PVC temporal
4. Smoke queries: row counts, último observation, último audit_log

## Riesgos

- Drift schema: validar versión migrate antes de restore
- Drill cuesta GB de transferencia → ejecutar fuera de hora pico
- Cluster efímero queda colgado si falla cleanup → TTL forzado

## Testing

- Ejecutar drill manualmente → reporte verde
- Sabotaje: corromper último WAL → drill falla → notificación → reporte rojo
