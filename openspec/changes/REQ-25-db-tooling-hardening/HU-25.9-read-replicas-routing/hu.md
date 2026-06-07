# HU-25.9-read-replicas-routing

**Origen:** `REQ-25-db-tooling-hardening`
**Persona:** platform-engineer
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** plataforma con carga creciente de lecturas
**Quiero** una read replica activa + router en pgx (writes → primary, reads pesados → replica)
**Para** escalar lecturas sin tocar primary

## Criterios de aceptación

### Escenario 1: Replica configurada

```gherkin
Dado que existe primary + 1 read replica con streaming replication
Cuando consulto `SELECT pg_is_in_recovery()` en replica
Entonces devuelve `t` (true)
Y `SELECT pg_last_wal_replay_lsn()` está cerca del primary
```

### Escenario 2: Router pgx

```gherkin
Dado que app tiene `DOMAIN_DATABASE_URL` primary y `DOMAIN_DATABASE_URL_READONLY` replica
Y existe helper `db.Read(ctx, fn)` y `db.Write(ctx, fn)`
Cuando se invoca `db.Read(ctx, q)` para queries SELECT pesados
Entonces se usa replica pool
Cuando se invoca `db.Write(ctx, q)` o queries con BEGIN
Entonces se usa primary
```

### Escenario 3: Replication lag monitoring

```gherkin
Dado que existe cron 30s monitoring
Cuando consulta `SELECT EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp())` en replica
Entonces métrica `domain_db_replication_lag_seconds` se publica
Y alerta si lag >5s sostenido 2min
```

### Escenario 4: Fallback a primary si lag alto

```gherkin
Dado que lag >10s detectado
Cuando `db.Read(ctx, fn)` se invoca
Entonces el router temporalmente routea a primary (degradation)
Y se logea warning
Y cuando lag <2s vuelve a replica
```

### Escenario 5: Stale-read tolerance config

```gherkin
Dado que existe `db.ReadFresh(ctx, fn)` para reads que NO toleran stale
Cuando se invoca
Entonces se usa primary directo
Y `db.Read(ctx, fn)` está documentado como tolerant de stale-read <2s
```

### Escenario 6: Replica usage en queries pesadas declarativa

```gherkin
Dado que HU-03.7 global search es una query pesada
Cuando se ejecuta
Entonces internamente usa `db.Read` → routea replica
Y la métrica `domain_db_replica_queries_total` aumenta
```

## Análisis breve

- **Qué pide:** replica streaming + 2 pools pgx + router helpers + lag monitoring + fallback
- **Esfuerzo:** M
- **Riesgos:** stale reads inadvertidos rompen UX; replica caída sin fallback rompe lecturas
