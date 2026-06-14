# Tasks: issue-25.2-pg-stat-statements

- [x] **ps-001**: postgresql.conf shared_preload + auto_explain params
- [x] **ps-002**: Migration CREATE EXTENSION pg_stat_statements
- [x] **ps-003**: Schema domain_query_stats_history partitioned weekly
- [x] **ps-004**: Cron analyzer 5min
- [x] **ps-005**: Cron snapshot+reset weekly
- [x] **ps-006**: Métricas Prometheus slow queries
- [x] **ps-007**: Notif nueva query >500ms
- [x] **ps-008**: Endpoint GET /admin/db/slow-queries
- [x] **ps-009**: Log shipper auto_explain JSON a Loki
- [x] **test-001**: Extensions cargadas
- [x] **test-002**: Slow query simulated detected
- [x] **test-003**: Alert nueva query
- [x] **test-004**: Reset weekly preserva snapshot
- [x] **docs-001**: `docs/db/slow-queries.md`
