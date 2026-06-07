# HU-25.8-resource-limits-timeouts

**Origen:** `REQ-25-db-tooling-hardening`
**Persona:** platform-engineer
**Prioridad tentativa:** alta
**Tipo:** hardening

## Historia de usuario

**Como** operador
**Quiero** límites strict en queries (statement_timeout, lock_timeout, idle_in_transaction)
**Para** que una query lenta no tumbe el primary y que TLS sea obligatorio

## Criterios de aceptación

### Escenario 1: statement_timeout 30s

```gherkin
Dado que `ALTER ROLE app_user SET statement_timeout = '30s'`
Y conexión usa app_user
Cuando se ejecuta query que tarda 35s
Entonces Postgres aborta con error `canceling statement due to statement timeout`
Y la conexión queda usable
```

### Escenario 2: lock_timeout 10s

```gherkin
Dado que `ALTER ROLE app_user SET lock_timeout = '10s'`
Cuando una query espera lock >10s
Entonces aborta sin hold prolongado
Y app retry con backoff (o sale claro)
```

### Escenario 3: idle_in_transaction_session_timeout 60s

```gherkin
Dado que app abre tx y no commitea por 90s (bug)
Cuando supera 60s
Entonces Postgres cierra la conexión con `terminating connection due to idle-in-transaction timeout`
Y los locks se liberan
```

### Escenario 4: connection limit por role

```gherkin
Dado que `ALTER ROLE app_user CONNECTION LIMIT 200`
Cuando 201va conexión intenta
Entonces error "too many connections for role"
Y métricas Prometheus alertan
```

### Escenario 5: work_mem calibrado

```gherkin
Dado que postgresql.conf:
  shared_buffers = 25% RAM
  work_mem = 16MB (per-operation)
  maintenance_work_mem = 256MB
  effective_cache_size = 75% RAM
  random_page_cost = 1.1 (SSD)
Cuando queries con sort/hash corren
Entonces no spillean a disco salvo en casos extremos
```

### Escenario 6: TLS mandatory verify-full prod

```gherkin
Dado que producción tiene `ssl = on` y `ssl_min_protocol_version = 'TLSv1.2'`
Y app usa `sslmode=verify-full` con CA pinning
Cuando un cliente intenta sin SSL
Entonces conexión rechazada por pg_hba.conf `hostnossl ... reject`
Y verify-full valida cert hostname
```

### Escenario 7: pg_hba.conf hardening

```gherkin
Dado que pg_hba.conf production:
  hostssl all app_user 10.0.0.0/16 scram-sha-256
  hostssl all app_migrator 10.0.10.0/24 scram-sha-256
  hostnossl all all all reject
Cuando conexión sin SSL llega
Entonces reject
Y conexión sin SCRAM-SHA-256 → reject
```

## Análisis breve

- **Qué pide:** timeouts ROLE-level + connection limits + work_mem calibration + TLS verify-full + pg_hba hardening
- **Esfuerzo:** S
- **Riesgos:** timeouts agresivos rompen jobs legítimos largos; certs misconfigured rompen conexión
