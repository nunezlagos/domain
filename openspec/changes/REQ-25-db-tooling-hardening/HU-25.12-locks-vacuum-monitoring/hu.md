# HU-25.12-locks-vacuum-monitoring

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** media
**Tipo:** tooling

## Historia de usuario

**Como** operador
**Quiero** monitoring de lock waits, autovacuum activity, bloat de tablas y advisor de índices
**Para** detectar issues antes que pages

## Criterios de aceptación

### Escenario 1: Lock waits monitoring

```gherkin
Dado que existe métrica `domain_db_lock_waits_total{wait_type,table}`
Cuando una query espera lock >5s
Entonces se publica evento + métrica incrementa
Y alerta si `lock_waits` >10 en 1min sostenido
```

### Escenario 2: Autovacuum activity

```gherkin
Dado que `pg_stat_user_tables` expone `last_autovacuum`, `n_dead_tup`, `n_live_tup`
Cuando cron 5min query
Entonces publica:
  - `domain_db_table_dead_tuples{table}`
  - `domain_db_table_last_autovacuum_age_seconds{table}`
Y alerta si dead_tup > live_tup * 0.5 sostenido 1h (vacuum no progresa)
```

### Escenario 3: Bloat detection

```gherkin
Dado que existe query bloat estándar (postgres wiki)
Cuando cron daily
Entonces calcula bloat % por table/index
Y publica `domain_db_table_bloat_ratio{table}`
Y alerta si >50% bloat
```

### Escenario 4: Index advisor pg_qualstats

```gherkin
Dado que `CREATE EXTENSION pg_qualstats`
Cuando cron weekly analyze
Entonces genera reporte de:
  - queries con seq scan en tablas grandes
  - propuesta de índice (qual + cardinality)
  - estimación benefit/cost
Y se publica en `docs/db/index-suggestions-YYYY-MM.md`
```

### Escenario 5: Connection state monitoring

```gherkin
Dado que `pg_stat_activity`
Cuando cron 30s
Entonces publica gauges:
  - `domain_db_connections_active`
  - `domain_db_connections_idle`
  - `domain_db_connections_idle_in_transaction`
  - `domain_db_longest_query_seconds`
Y alerta si idle_in_transaction >10 o longest_query >60s
```

### Escenario 6: Bloat-related VACUUM tuning recommendations

```gherkin
Dado que tabla tiene high write churn y bloat
Cuando advisor weekly
Entonces propone `ALTER TABLE foo SET (autovacuum_vacuum_scale_factor = 0.05)` para vacuum más agresivo
```

## Análisis breve

- **Qué pide:** métricas Prometheus + cron advisor + alertas + pg_qualstats + tuning recommendations
- **Esfuerzo:** M
- **Riesgos:** falsos positivos en bloat (tablas legítimamente sparse); pg_qualstats overhead
