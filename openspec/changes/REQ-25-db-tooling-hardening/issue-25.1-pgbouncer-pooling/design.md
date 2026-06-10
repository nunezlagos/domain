# Design: issue-25.1-pgbouncer-pooling

## Decisión arquitectónica

**Pooler:** PgBouncer 1.22+ (estable, simple).
**Mode:** `transaction` (default; libera server-conn al COMMIT).
**Auth:** SCRAM-SHA-256.
**HA:** 2+ réplicas detrás de Service + PDB minAvailable=1.
**Listener address:** ClusterIP no público.

## Alternativas descartadas

- **Pgcat:** más nuevo, multi-shard pero menos battle-tested
- **PgPool-II:** demasiadas features (replication, load-balancing internal) que ya cubrimos con issue-25.9
- **No pooler / pgx pool only:** no escala >50 pods sin saturar primary

## pgbouncer.ini

```ini
[databases]
domain = host=postgres-primary port=5432 dbname=domain

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = scram-sha-256
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = transaction
max_client_conn = 2000
default_pool_size = 20
reserve_pool_size = 5
reserve_pool_timeout = 3
server_idle_timeout = 600
query_wait_timeout = 120
ignore_startup_parameters = extra_float_digits,search_path
admin_users = pgbouncer_admin
stats_users = pgbouncer_stats
```

## pgx config en app

```go
config, _ := pgxpool.ParseConfig(databaseURL)
config.MaxConns = 10  // per pod
config.MinConns = 2
// importante para transaction-pooling:
config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
```

## Manifests (helm)

```yaml
# templates/pgbouncer-deployment.yaml
replicas: 2
podDisruptionBudget: { minAvailable: 1 }
sidecars:
  - name: exporter
    image: prometheuscommunity/pgbouncer-exporter:0.7
```

## TDD plan

1. 200 conns app, primary <50
2. Failover 1 pod
3. Prepared stmt funciona end-to-end
4. SET LOCAL en tx persiste durante tx, no después
5. LISTEN/NOTIFY error explícito documentado
6. Exporter `/metrics` accesible
