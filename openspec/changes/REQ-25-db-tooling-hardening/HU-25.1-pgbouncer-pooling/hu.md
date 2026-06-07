# HU-25.1-pgbouncer-pooling

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** operador escalando pods de Domain
**Quiero** PgBouncer en transaction-pooling delante de Postgres
**Para** soportar muchos pods sin saturar `max_connections` del primary

## Criterios de aceptación

### Escenario 1: PgBouncer entre app y Postgres

```gherkin
Dado que existe Service `domain-pgbouncer` en el cluster
Cuando los pods de Domain se conectan
Entonces apuntan a `pgbouncer:6432` (no directo a postgres:5432)
Y PgBouncer mantiene `max_client_conn=2000` y `default_pool_size=20` por usuario
Y el primary Postgres ve <100 conexiones server-side aun con 100+ pods app
```

### Escenario 2: Transaction-pooling

```gherkin
Dado que `pool_mode=transaction`
Cuando un pod hace BEGIN; ... ; COMMIT
Entonces obtiene server-conn al BEGIN y la libera al COMMIT
Y prepared statements requieren `server_reset_query_always=0` + protocolo correcto (pgx con `disable_prepared_statements` o `pg_prepare_threshold=0`)
```

### Escenario 3: HA con 2 réplicas

```gherkin
Dado que existen 2 réplicas de PgBouncer detrás de un Service
Cuando una pod cae
Entonces el ELB drena conexiones gracefully
Y nuevas conexiones van a la sana
Y el cliente reintenta con backoff (pgx maneja)
```

### Escenario 4: Monitoring exporter

```gherkin
Dado que existe `prometheus-pgbouncer-exporter` sidecar o standalone
Cuando scrape Prometheus
Entonces hay métricas:
  | métrica                                | descripción          |
  | pgbouncer_pools_client_active          | clientes activos     |
  | pgbouncer_pools_client_waiting         | en cola              |
  | pgbouncer_pools_server_active          | server conns busy    |
  | pgbouncer_stats_avg_query_time         | latencia promedio    |
Y se alertan: `client_waiting > 50` por 1min, `server_active / pool_size > 0.9` por 5min
```

### Escenario 5: Configuración por env

```gherkin
Dado que helm values incluyen:
  pgbouncer:
    poolMode: transaction
    maxClientConn: 2000
    defaultPoolSize: 20
    reservePoolSize: 5
    serverIdleTimeout: 600
    queryWaitTimeout: 120
Cuando deploy con valores override
Entonces PgBouncer levanta con esos parámetros
```

## Análisis breve

- **Qué pide:** PgBouncer container + Service + HA + exporter + helm config
- **Esfuerzo:** M
- **Riesgos:** prepared statements en transaction-pooling rompen si no se configura pgx; SET/LISTEN no soportados en transaction mode
