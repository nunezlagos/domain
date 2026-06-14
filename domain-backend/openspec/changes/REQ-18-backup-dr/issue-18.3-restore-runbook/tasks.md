# Tasks: issue-18.3-restore-runbook

- [x] **rb-001**: Redactar `docs/runbooks/restore.md` con 6 secciones
- [x] **rb-002**: Script `scripts/drill/restore-drill.sh` idempotente con logs
- [x] **rb-003**: Kubernetes CronJob mensual + Job template
- [x] **rb-004**: RBAC SA con acceso read-only a S3 prod backups
- [x] **rb-005**: TTL forzado para cluster efímero (2h)
- [x] **rb-006**: Métrica `domain_restore_drill_*` exportada
- [x] **rb-007**: Auto-commit reporte drill al repo
- [x] **test-001**: Drill manual end-to-end verde
- [x] **sabotaje-001**: Corromper WAL fixture → drill rojo + notificación
- [x] **docs-001**: Post-mortem template en runbook
