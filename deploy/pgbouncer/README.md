# PgBouncer setup — Domain

PgBouncer multiplexa conexiones DB para que muchos pods Domain compartan
pocas conexiones reales a Postgres. Modo transaction-pool compatible con
SET LOCAL (HU-25.5 RLS).

## Deploy K8s (sidecar opcional)

PgBouncer puede correr como:
1. **Sidecar** en cada pod Domain (latencia ~0, footprint mayor)
2. **Service externo** dedicado (más escalable, ~1ms latency)

Recomendación prod: opción 2.

```bash
helm upgrade --install pgbouncer-domain bitnami/pgbouncer \
  --set auth.database=domain \
  --set auth.username=pgbouncer \
  --set auth.existingSecret=pgbouncer-creds \
  --set service.ports.pgbouncer=6432
```

## Userlist

`/etc/pgbouncer/userlist.txt` formato `"user" "password"`:

```
"app_user" "scram-sha-256$..."
"app_admin" "scram-sha-256$..."
"app_readonly" "scram-sha-256$..."
```

Generar con:

```bash
psql -U postgres -c "SELECT rolname, rolpassword FROM pg_authid WHERE rolname IN ('app_user','app_admin');"
```

## DSN para Domain

En Helm values:

```yaml
existingSecrets:
  database:
    name: domain-db
    # En lugar de host=postgres-primary, usar pgbouncer
    keyURL: "postgres://app_user:***@pgbouncer-domain:6432/domain?sslmode=verify-full"
    keyAuthURL: "postgres://app_admin:***@pgbouncer-domain:6432/domain?sslmode=verify-full"
```

## Incompatibilidades con transaction-pool

Domain NO usa:
- `PREPARE` statements explícitos (pgx prepara con extended protocol, ok)
- `SET` sin `SET LOCAL` (todas las variables app.* van con SET LOCAL en tx)
- `LISTEN/NOTIFY` (sería cross-tx; usar Postgres pubsub directo si necesario)

Si una feature futura requiere session pool en lugar de transaction, agregar
`pool_mode=session` para ese DB en la conf — pero pierde la multiplexación
fuerte.

## Métricas Prometheus

PgBouncer expone stats vía `SHOW STATS`. Usar
[pgbouncer_exporter](https://github.com/jbub/pgbouncer_exporter) como sidecar:

```yaml
- name: pgbouncer-exporter
  image: jbub/pgbouncer_exporter:latest
  env:
    - name: PGBOUNCER_EXPORTER_HOST
      value: "127.0.0.1"
    - name: PGBOUNCER_EXPORTER_PORT
      value: "6432"
  ports:
    - name: metrics
      containerPort: 9127
```

Alertas:
- `pgbouncer_pools_cl_waiting > 10` por > 5min → bajo pool
- `pgbouncer_pools_sv_active / max_db_connections > 0.8` → cerca del cap
