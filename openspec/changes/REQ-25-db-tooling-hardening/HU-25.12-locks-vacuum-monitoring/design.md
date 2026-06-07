# Design: HU-25.12-locks-vacuum-monitoring

## Queries clave

### Lock waits
```sql
SELECT count(*), pg_blocking_pids(pid), wait_event_type, wait_event
FROM pg_stat_activity
WHERE wait_event_type IN ('Lock','LWLock')
  AND now() - state_change > interval '5 seconds';
```

### Autovacuum stats
```sql
SELECT relname, n_live_tup, n_dead_tup,
  EXTRACT(EPOCH FROM now() - last_autovacuum) AS last_av_age,
  n_dead_tup::float / nullif(n_live_tup, 0) AS dead_ratio
FROM pg_stat_user_tables
ORDER BY dead_ratio DESC NULLS LAST;
```

### Bloat (Postgres Wiki query)
```sql
-- standard bloat query, demasiado largo para incluir aquí
-- ver https://wiki.postgresql.org/wiki/Show_database_bloat
```

### Index suggestions (pg_qualstats)
```sql
SELECT qual_table_name, qual_column_name, count(*) AS occurrences,
  avg(execution_count) AS avg_exec
FROM pg_qualstats_indexes_view
GROUP BY qual_table_name, qual_column_name
ORDER BY occurrences * avg_exec DESC LIMIT 20;
```

## Worker

```go
// cmd/domain-mcp db-monitor
// every 5min: locks + autovacuum
// every 24h: bloat
// every 7d: index advisor → write docs/db/index-suggestions-YYYY-MM-DD.md
```

## Métricas

```
domain_db_lock_waits_total{wait_type,table}
domain_db_table_dead_tuples{table}
domain_db_table_last_autovacuum_age_seconds{table}
domain_db_table_bloat_ratio{table}
domain_db_index_bloat_ratio{index}
domain_db_connections_state{state}
domain_db_longest_query_seconds
```

## TDD plan

1. Lock simulado → métrica + alert
2. Autovacuum stuck → métrica + alert
3. Bloat fixture → reporte
4. pg_qualstats reporte semanal
5. Connection idle_in_tx detection
