# Design: issue-25.2-pg-stat-statements

## Postgres config

```
shared_preload_libraries = 'pg_stat_statements,auto_explain'
pg_stat_statements.max = 10000
pg_stat_statements.track = all
pg_stat_statements.save = on

auto_explain.log_min_duration = 100   # ms
auto_explain.log_analyze = true
auto_explain.log_buffers = true
auto_explain.log_format = json
auto_explain.log_verbose = false
auto_explain.log_nested_statements = true
```

## Schema histórico

```sql
CREATE TABLE domain_query_stats_history (
  snapshot_at TIMESTAMPTZ NOT NULL,
  queryid BIGINT NOT NULL,
  query_normalized TEXT NOT NULL,
  calls BIGINT,
  total_exec_time DOUBLE PRECISION,
  mean_exec_time DOUBLE PRECISION,
  max_exec_time DOUBLE PRECISION,
  rows BIGINT,
  PRIMARY KEY (snapshot_at, queryid)
) PARTITION BY RANGE (snapshot_at);
```

## Cron worker

```go
// cmd/domain-mcp slow-query-analyzer
// runs every 5min
rows := db.Query(`
  SELECT queryid, query, calls, mean_exec_time, total_exec_time, rows
  FROM pg_stat_statements
  WHERE mean_exec_time > $1 OR (calls > $2 AND mean_exec_time > $3)
  ORDER BY total_exec_time DESC LIMIT 100
`, 100.0, 100, 50.0)

for r := range rows {
  metrics.SlowQuery(r.queryid).Inc()
  if r.is_new_and_slow { notifications.Enqueue("slow_query_alert", r) }
}
```

## TDD plan

1. Extensiones cargadas (SELECT * FROM pg_stat_statements works)
2. Slow query simulada detectada (`pg_sleep(0.2)` aparece en top)
3. Auto_explain logea EXPLAIN ANALYZE en log file
4. Alerta nueva query >500ms
5. Reset semanal preserva histórico
6. Endpoint /admin/db/slow-queries con RBAC
