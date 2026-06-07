# HU-25.7-pgaudit-db-level

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** media
**Tipo:** hardening

## Historia de usuario

**Como** compliance/security officer
**Quiero** auditoría a nivel Postgres (paralelo al audit_log de app)
**Para** tener evidencia indiscutible (PostgreSQL nativo) de DDL y operaciones sensibles, aún si la app es comprometida

## Eventos a capturar

- `DDL` — CREATE/ALTER/DROP en cualquier role
- `ROLE` — CREATE ROLE, GRANT, REVOKE
- `READ` — SELECT en tablas sensibles (`secrets`, `audit_log`, `billing*`)
- `WRITE` — INSERT/UPDATE/DELETE en `secrets`, `subscriptions`, `custom_roles`
- `FUNCTION` — invocaciones de funciones específicas

## Criterios de aceptación

### Escenario 1: pgaudit habilitado

```gherkin
Dado que postgresql.conf incluye:
  shared_preload_libraries = '...,pgaudit'
  pgaudit.log = 'ddl, role, write'
  pgaudit.log_relation = on
  pgaudit.log_catalog = off
Y CREATE EXTENSION pgaudit
Cuando reinicio Postgres
Entonces pgaudit está activo
Y `SELECT * FROM pg_extension WHERE extname='pgaudit'` devuelve 1 fila
```

### Escenario 2: DDL capturado

```gherkin
Dado que app_migrator ejecuta `CREATE TABLE foo`
Cuando se procesa
Entonces el log Postgres contiene línea:
  AUDIT: SESSION,1,1,DDL,CREATE TABLE,TABLE,foo,...
Y se shippea a Loki/CloudWatch
```

### Escenario 3: SELECT en secrets logged

```gherkin
Dado que `pgaudit.log_object = 'public.secrets'` (object audit)
Cuando un app_user SELECT * FROM secrets
Entonces se logea con statement + parámetros redactados
```

### Escenario 4: Logs shippeados separados

```gherkin
Dado que pgaudit logs marcan `AUDIT:` prefix
Cuando filebeat/promtail procesa
Entonces los routea a label `audit=true` y bucket Loki o índice CloudWatch separado
Y retention 7 años (compliance)
```

### Escenario 5: No log de password

```gherkin
Dado que se ejecuta `CREATE ROLE x WITH PASSWORD 'secret'`
Cuando pgaudit logea
Entonces el password NO aparece en clear
Y se substitute por `<REDACTED>`
```

## Análisis breve

- **Qué pide:** pgaudit extension + config + log shipper + retention compliance
- **Esfuerzo:** S
- **Riesgos:** log volume grande; performance overhead 5-10% si log de SELECT/WRITE muy amplio
