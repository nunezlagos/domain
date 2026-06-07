# Design: HU-25.8-resource-limits-timeouts

## Per-role settings

```sql
ALTER ROLE app_user SET statement_timeout = '30s';
ALTER ROLE app_user SET lock_timeout = '10s';
ALTER ROLE app_user SET idle_in_transaction_session_timeout = '60s';
ALTER ROLE app_user CONNECTION LIMIT 200;

ALTER ROLE app_migrator SET statement_timeout = '15min';
ALTER ROLE app_migrator SET lock_timeout = '30s';
ALTER ROLE app_migrator CONNECTION LIMIT 5;

ALTER ROLE app_admin SET statement_timeout = '5min';
ALTER ROLE app_admin CONNECTION LIMIT 10;

ALTER ROLE app_readonly SET statement_timeout = '2min';
ALTER ROLE app_readonly CONNECTION LIMIT 50;
```

## postgresql.conf (prod, 8GB RAM example)

```
shared_buffers = 2GB
work_mem = 16MB
maintenance_work_mem = 256MB
effective_cache_size = 6GB
random_page_cost = 1.1
checkpoint_completion_target = 0.9
wal_buffers = 16MB
min_wal_size = 1GB
max_wal_size = 4GB
max_connections = 300

ssl = on
ssl_min_protocol_version = 'TLSv1.2'
ssl_cert_file = '/var/lib/postgresql/certs/server.crt'
ssl_key_file = '/var/lib/postgresql/certs/server.key'
ssl_ca_file = '/var/lib/postgresql/certs/ca.crt'
```

## pg_hba.conf (prod)

```
# TYPE  DATABASE  USER          ADDRESS         METHOD
hostssl   domain    app_user      10.0.0.0/16     scram-sha-256
hostssl   domain    app_migrator  10.0.10.0/24    scram-sha-256
hostssl   domain    app_admin     10.0.20.0/24    scram-sha-256
hostssl   domain    app_readonly  10.0.30.0/24    scram-sha-256
hostnossl all       all           all              reject
host      all       all           all              reject
```

## App pgx URL

```
postgres://app_user:<pwd>@pgbouncer:6432/domain?sslmode=verify-full&sslrootcert=/etc/pg-certs/ca.crt
```

## TDD plan

1. 31s query → timeout
2. Lock 11s → aborted
3. idle_in_tx 60s → closed
4. 201va conn → too many
5. sslmode=disable → reject
6. Cert hostname mismatch → reject
7. cert expiry monitor alert
