# Tasks: issue-25.12-locks-vacuum-monitoring

- [ ] **lvm-001**: CREATE EXTENSION pg_qualstats
- [ ] **lvm-002**: Worker `domain-mcp db-monitor`
- [ ] **lvm-003**: Cron 5min snapshot locks + autovacuum
- [ ] **lvm-004**: Cron daily bloat calc
- [ ] **lvm-005**: Cron weekly index advisor + reporte md
- [ ] **lvm-006**: Métricas Prometheus
- [ ] **lvm-007**: PrometheusRule alertas
- [ ] **test-001**: Lock simulated alert
- [ ] **test-002**: Autovacuum stuck alert
- [ ] **test-003**: Bloat fixture reporte
- [ ] **test-004**: pg_qualstats weekly
- [ ] **test-005**: Connection idle_in_tx detection
- [ ] **docs-001**: `docs/db/monitoring.md`
