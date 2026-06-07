# HU-25.2-pg-stat-statements

**Origen:** `REQ-25-db-tooling-hardening`
**Persona:** platform-engineer
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** operador/SRE
**Quiero** `pg_stat_statements` + `auto_explain` + alertas slow query
**Para** detectar queries problemáticas antes que pages

## Criterios de aceptación

### Escenario 1: Extensiones habilitadas

```gherkin
Dado que postgresql.conf incluye `shared_preload_libraries = 'pg_stat_statements,auto_explain'`
Y `pg_stat_statements.max = 10000`
Y `pg_stat_statements.track = all`
Y `auto_explain.log_min_duration = 100`
Y `auto_explain.log_analyze = true`
Y `auto_explain.log_buffers = true`
Cuando reinicio Postgres
Entonces las extensiones están activas
Y `SELECT * FROM pg_stat_statements` devuelve filas
```

### Escenario 2: Slow query alert

```gherkin
Dado que existe cron `slow-query-report` cada 5min
Cuando se ejecuta
Entonces consulta:
  ```sql
  SELECT query, calls, mean_exec_time, p95
  FROM pg_stat_statements
  WHERE mean_exec_time > 100 OR (calls > 100 AND mean_exec_time > 50)
  ```
Y publica métrica `domain_db_slow_queries_total`
Y si hay query nueva con mean_exec_time > 500ms → notif al canal SRE
```

### Escenario 3: Auto-explain logs queries lentas

```gherkin
Dado que una query supera 100ms
Cuando se ejecuta
Entonces se logea su EXPLAIN ANALYZE en logs Postgres
Y se exporta via filebeat/promtail a Loki/CloudWatch
Y se puede consultar con LogQL/Insights
```

### Escenario 4: Reset histórico semanal

```gherkin
Dado que pg_stat_statements acumula histórico desde último reset
Cuando ejecuta cron weekly `pg_stat_statements_reset()`
Entonces se hace snapshot a tabla `domain_query_stats_history` antes de resetear
Y se preserva trend semanal
```

### Escenario 5: Top-N endpoint admin

```gherkin
Dado que admin platform consulta GET /admin/db/slow-queries?limit=20
Cuando se procesa
Entonces devuelve top-20 por mean_exec_time + calls + total_exec_time
Y incluye normalized query (sin valores literal)
Y RBAC enforce platform_admin role
```

## Análisis breve

- **Qué pide:** extensiones habilitadas + cron analyzer + alertas + historial + endpoint admin
- **Esfuerzo:** S
- **Riesgos:** overhead minor (~1-3%) aceptable; auto_explain log spam si threshold mal calibrado
