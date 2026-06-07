# HU-25.6-least-privilege-roles

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** alta
**Tipo:** hardening

## Historia de usuario

**Como** SRE/Security
**Quiero** roles Postgres con least-privilege grants
**Para** que ni un SQL injection ni un compromise del pod permita escalar a admin DB

## Roles

| role | grants | uso |
|------|--------|-----|
| `app_user` | SELECT/INSERT/UPDATE/DELETE en tables del schema `public` excepto `audit_log` (solo INSERT) | runtime de la app pods |
| `app_admin` | + BYPASSRLS, + DELETE en audit_log (con audit) | batch jobs admin specific |
| `app_migrator` | CREATE/ALTER/DROP en schema (DDL) | solo CI/CD migration step |
| `app_readonly` | SELECT only | reportes, analytics tooling |
| `pgbouncer_admin` | reload pgbouncer config | admin pgbouncer |
| `pgbouncer_stats` | SHOW STATS en pgbouncer | monitoring exporter |

## Criterios de aceptación

### Escenario 1: app_user no puede DDL

```gherkin
Dado que conexión app usa role `app_user`
Cuando intenta `CREATE TABLE foo (id int)` o `DROP TABLE users` o `ALTER TABLE ...`
Entonces error "permission denied"
Y NO se ejecuta
```

### Escenario 2: app_user no puede UPDATE audit_log

```gherkin
Dado que app_user
Cuando intenta `UPDATE audit_log SET action = 'hide'`
Entonces error "permission denied for table audit_log"
Y misma protección para DELETE FROM audit_log
```

### Escenario 3: app_migrator usado solo en CI

```gherkin
Dado que migration job (HU-24.1 helm hook) corre
Cuando se ejecuta `migrate up`
Entonces usa role `app_migrator` con password rotado per-run vía secret K8s
Y al terminar, conexión cerrada
Y app pods NUNCA usan app_migrator credentials
```

### Escenario 4: app_readonly para reportes

```gherkin
Dado que existe tooling reports/analytics
Cuando se conecta con app_readonly
Entonces SELECT funciona
Y cualquier INSERT/UPDATE/DELETE/DDL → "permission denied"
```

### Escenario 5: REVOKE TRUNCATE

```gherkin
Dado que app_user
Cuando intenta `TRUNCATE TABLE observations`
Entonces error (TRUNCATE no es DELETE, requiere TRUNCATE privilege)
Y app_user NO tiene TRUNCATE privilege
```

### Escenario 6: Public schema lockdown

```gherkin
Dado que Postgres por default da CREATE en public a PUBLIC role
Cuando aplicamos hardening
Entonces `REVOKE CREATE ON SCHEMA public FROM PUBLIC`
Y nadie puede crear tablas sin grant explícito
```

## Análisis breve

- **Qué pide:** definir 4-6 roles + grants per-table + REVOKE defaults peligrosos + tests
- **Esfuerzo:** S
- **Riesgos:** breaking deploys si rol no tiene grant que necesita; nuevas tablas en migrations requieren GRANT explícito
