# Tasks: HU-25.2-pg-stat-statements

- [ ] **ps-001**: postgresql.conf shared_preload + auto_explain params
- [ ] **ps-002**: Migration CREATE EXTENSION pg_stat_statements
- [ ] **ps-003**: Schema domain_query_stats_history partitioned weekly
- [ ] **ps-004**: Cron analyzer 5min
- [ ] **ps-005**: Cron snapshot+reset weekly
- [ ] **ps-006**: Métricas Prometheus slow queries
- [ ] **ps-007**: Notif nueva query >500ms
- [ ] **ps-008**: Endpoint GET /admin/db/slow-queries
- [ ] **ps-009**: Log shipper auto_explain JSON a Loki
- [ ] **test-001**: Extensions cargadas
- [ ] **test-002**: Slow query simulated detected
- [ ] **test-003**: Alert nueva query
- [ ] **test-004**: Reset weekly preserva snapshot
- [ ] **docs-001**: `docs/db/slow-queries.md`
