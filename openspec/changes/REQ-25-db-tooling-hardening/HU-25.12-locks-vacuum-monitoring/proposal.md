# Proposal: HU-25.12-locks-vacuum-monitoring

## Intención

Monitorear lock waits, autovacuum activity, bloat de tablas/índices, estado de conexiones, e implementar index advisor con `pg_qualstats`.

## Scope

**Incluye:**
- Métricas Prometheus: locks, autovacuum, bloat, connections
- Cron 5min: snapshot pg_stat_*
- Cron daily: bloat calculation
- Cron weekly: index advisor con pg_qualstats
- Alertas para casos críticos
- Reportes markdown auto-generados

**No incluye:**
- Auto-apply de index suggestions (manual review)
- VACUUM tuning automation (manual)

## Enfoque técnico

1. CREATE EXTENSION pg_qualstats + pg_stat_statements (HU-25.2)
2. Worker collector con queries known-good
3. Métrica exporter parallel a HU-17.1
4. Reportes markdown committed a repo

## Riesgos

- Bloat false positives: documentar excepciones manuales
- pg_qualstats overhead: deshabilitable per env

## Testing

- Métricas locks/autovacuum/bloat publicadas
- Lock wait simulado → alerta
- Autovacuum stuck simulado → alerta
- Index advisor weekly genera reporte
