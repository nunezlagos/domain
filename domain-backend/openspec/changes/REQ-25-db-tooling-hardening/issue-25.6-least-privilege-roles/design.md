# Design: issue-25.6-least-privilege-roles

## Roles SQL

```sql
-- migration 000XXX_create_roles.up.sql
CREATE ROLE app_user NOBYPASSRLS LOGIN PASSWORD :'app_user_password';
CREATE ROLE app_admin BYPASSRLS LOGIN PASSWORD :'app_admin_password';
CREATE ROLE app_migrator LOGIN PASSWORD :'app_migrator_password';
CREATE ROLE app_readonly NOBYPASSRLS LOGIN PASSWORD :'app_readonly_password';

-- revoke peligrosos defaults
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON DATABASE domain FROM PUBLIC;

GRANT CONNECT ON DATABASE domain TO app_user, app_admin, app_readonly, app_migrator;
GRANT USAGE ON SCHEMA public TO app_user, app_admin, app_readonly;
GRANT USAGE, CREATE ON SCHEMA public TO app_migrator;

-- grants tabla por tabla (template en helper)
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO app_readonly;
GRANT INSERT ON audit_log TO app_user;
REVOKE UPDATE, DELETE ON audit_log FROM app_user;
GRANT ALL ON ALL TABLES IN SCHEMA public TO app_admin;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user, app_admin;

-- default privileges (futuros objetos creados por app_migrator)
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT SELECT ON TABLES TO app_readonly;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO app_user;
```

## K8s Secrets

```
domain-db-app-user      → DOMAIN_DATABASE_URL (app pods)
domain-db-app-migrator  → solo Job migration hook
domain-db-app-readonly  → reports/analytics tooling
domain-db-app-admin     → batch admin tools
```

## Rol-tabla matrix (docs)

| tabla | app_user | app_readonly | app_admin |
|-------|----------|--------------|-----------|
| organizations | RWUD | R | RWUD |
| users | RWUD | R | RWUD |
| audit_log | I only | R | RWUD |
| observations | RWUD | R | RWUD |
| secrets | RWUD (con RLS) | R | RWUD (BYPASSRLS) |
| ... | ... | ... | ... |

## TDD plan

1. app_user DDL → denied
2. app_user UPDATE audit_log → denied
3. app_migrator DDL OK
4. app_readonly INSERT → denied
5. TRUNCATE app_user denied
6. Public lockdown (no CREATE for PUBLIC)
7. Nueva tabla via app_migrator → app_user puede CRUD por DEFAULT PRIVILEGES
