# Proposal: issue-25.9-read-replicas-routing

## Intención

1 read replica streaming + pgx con dos pools (primary + replica) + router helpers (`db.Read` / `db.Write` / `db.ReadFresh`) + monitoring de lag + fallback automático.

## Scope

**Incluye:**
- Helm chart soporta `postgresql.replica.enabled` y replica StatefulSet
- App config `DOMAIN_DATABASE_URL` (primary) y `DOMAIN_DATABASE_URL_READONLY` (replica)
- 2 pools pgx con health checks
- Helpers `db.Read(ctx, fn)`, `db.Write(ctx, fn)`, `db.ReadFresh(ctx, fn)`
- Lag monitoring cron 30s + Prometheus
- Auto-fallback a primary si lag >threshold
- Refactor queries pesadas (search global, analytics) a usar Read

**No incluye:**
- Multi-replica con load balancing (1 replica suficiente MVP)
- Logical replication / Citus (futuro si escala lo justifica)

## Enfoque técnico

1. Streaming replication async desde primary
2. pgx separate pool con `target_session_attrs=read-only` opcional
3. Router checks lag periódicamente y bandera in-memory
4. `Read` con fallback transparente

## Riesgos

- Stale reads: documentar `Read` tolerance + `ReadFresh` para no-tolerantes
- Replica caída: fallback a primary aceptable degradation
- Cost: 1 replica más infra; documentar en cost estimates

## Testing

- Replica recibe streaming
- db.Read va a replica
- db.Write va a primary
- Lag monitoring publica métrica
- Lag >10s → fallback a primary
- db.ReadFresh siempre primary
