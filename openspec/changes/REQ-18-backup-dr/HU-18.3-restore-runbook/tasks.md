# Tasks: HU-18.3-restore-runbook

- [ ] **rb-001**: Redactar `docs/runbooks/restore.md` con 6 secciones
- [ ] **rb-002**: Script `scripts/drill/restore-drill.sh` idempotente con logs
- [ ] **rb-003**: Kubernetes CronJob mensual + Job template
- [ ] **rb-004**: RBAC SA con acceso read-only a S3 prod backups
- [ ] **rb-005**: TTL forzado para cluster efímero (2h)
- [ ] **rb-006**: Métrica `domain_restore_drill_*` exportada
- [ ] **rb-007**: Auto-commit reporte drill al repo
- [ ] **test-001**: Drill manual end-to-end verde
- [ ] **sabotaje-001**: Corromper WAL fixture → drill rojo + notificación
- [ ] **docs-001**: Post-mortem template en runbook
