# Proposal: issue-25.2-pg-stat-statements

## Intención

Habilitar `pg_stat_statements` + `auto_explain` + cron analyzer + alertas para detectar queries lentas/regresiones de performance proactivamente.

## Scope

**Incluye:**
- `shared_preload_libraries = 'pg_stat_statements,auto_explain'`
- Migration `CREATE EXTENSION pg_stat_statements`
- Config auto_explain `log_min_duration=100`, `log_analyze=true`, `log_buffers=true`
- Cron 5min: query top-N slow + publish métricas
- Cron weekly: snapshot histórico + reset
- Alertas notif si nueva query >500ms p95
- Endpoint admin `/admin/db/slow-queries`

**No incluye:**
- Query plan visualization UI (futuro)
- Auto-tuning de queries (futuro)

## Enfoque técnico

1. Postgres config via ConfigMap/cloud DB parameter group
2. Cron Kubernetes Job worker
3. Métricas `domain_db_slow_queries_total` + `domain_db_query_p95_ms`
4. Tabla `domain_query_stats_history` particionada por semana

## Riesgos

- Overhead ~1-3% pg_stat_statements: aceptable
- auto_explain log spam: threshold 100ms calibrado
- Queries normalizadas pueden agrupar mal: aceptable, mejor que nada

## Testing

- Extensiones cargadas tras restart
- Slow query simulada (pg_sleep 200ms) detectada
- Alerta dispara con nueva query >500ms
- Endpoint top-N RBAC
- Reset semanal preserva histórico
