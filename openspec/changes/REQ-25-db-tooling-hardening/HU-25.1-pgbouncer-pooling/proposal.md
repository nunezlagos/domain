# Proposal: HU-25.1-pgbouncer-pooling

## Intención

Insertar PgBouncer (transaction-pooling) entre los pods de Domain y Postgres para escalar conexiones, evitar saturar `max_connections` del primary, y dar capa de observabilidad/control de pool.

## Scope

**Incluye:**
- Deployment PgBouncer en Helm chart (HU-24.1) con 2+ réplicas
- Service ClusterIP `domain-pgbouncer:6432`
- ConfigMap con `pool_mode=transaction`, sizes parametrizables
- pgx config en app: `disable_prepared_statements=false` con `statement_cache_mode=describe`
- Exporter Prometheus con métricas pool
- Alertas básicas en `client_waiting`, `server_active/pool_size`
- Health endpoint para readiness/liveness K8s

**No incluye:**
- PgPool/Pgcat (PgBouncer es estándar)
- Session pooling mode (transaction es el modo objetivo)
- TLS interno entre app-pgbouncer-postgres (HU-25.8 cubre TLS)

## Enfoque técnico

1. Image `edoburu/pgbouncer` o build custom desde upstream
2. Auth: `auth_type=scram-sha-256`, userlist generado desde Secret
3. NOTIFY/LISTEN: detectar uso en app y migrar a polling o usar session-pool secundario para esos casos
4. Prepared statements: pgx v5 con `DefaultQueryExecMode = QueryExecModeDescribeExec`

## Riesgos

- Prepared stmts caching roto en transaction mode → pgx config correcta + integration test
- SET app.current_org (HU-25.5 RLS) en transaction mode: debe hacerse al inicio de cada tx (`SET LOCAL`)
- Single point of failure si 1 réplica → mínimo 2 réplicas + PDB

## Testing

- Integration: 200 conns app → primary ve <50
- Failover: kill 1 PgBouncer pod → otras absorben sin error visible
- Prepared stmt funciona (pgx mode correcto)
- LISTEN/NOTIFY: documentar que NO funciona en transaction mode, usar polling
- Métricas exporter visibles en /metrics
