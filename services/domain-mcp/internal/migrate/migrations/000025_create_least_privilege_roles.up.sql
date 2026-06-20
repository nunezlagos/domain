-- migration: create_least_privilege_roles
-- author: mnunez@saargo.com
-- issue: HU-25.6
-- description: 4 roles least-privilege + REVOKE public defaults
-- breaking: false
-- estimated_duration: <1s
--
-- IMPORTANTE: passwords se setean post-deploy via `domain user create-role` o ESO.
-- Esta migration NO setea password (sería leak en logs migración).
--
-- Roles:
--   app_user      — runtime CRUD per-table grants (NOBYPASSRLS)
--   app_admin     — batch jobs, BYPASSRLS para RLS HU-25.5
--   app_migrator  — DDL via golang-migrate (CREATE/ALTER/DROP)
--   app_readonly  — reports/analytics (solo SELECT)

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_user') THEN
    CREATE ROLE app_user NOBYPASSRLS NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_admin') THEN
    CREATE ROLE app_admin BYPASSRLS NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_migrator') THEN
    CREATE ROLE app_migrator NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_readonly') THEN
    CREATE ROLE app_readonly NOBYPASSRLS NOLOGIN;
  END IF;
END $$;

-- REVOKE peligrosos defaults sobre PUBLIC role
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
-- GRANT CONNECT específico a la database se aplica dinámicamente con current_database()
-- para soportar dev (database "domain") y testcontainers (database "test").
DO $$
BEGIN
  EXECUTE format('REVOKE ALL ON DATABASE %I FROM PUBLIC', current_database());
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO app_user, app_admin, app_readonly, app_migrator', current_database());
END $$;

GRANT USAGE ON SCHEMA public TO app_user, app_admin, app_readonly;
GRANT USAGE, CREATE ON SCHEMA public TO app_migrator;

-- app_user CRUD sobre tablas existentes (excepto audit_log: solo INSERT)
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
REVOKE UPDATE, DELETE ON audit_log FROM app_user;

-- app_admin todo (BYPASSRLS para batch jobs admin)
GRANT ALL ON ALL TABLES IN SCHEMA public TO app_admin;

-- app_readonly solo SELECT
GRANT SELECT ON ALL TABLES IN SCHEMA public TO app_readonly;

-- Sequences (UUID PKs no usan sequences, pero BIGSERIAL sí: audit_log, cost_logs)
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user, app_admin;

-- DEFAULT PRIVILEGES: futuras tablas creadas por app_migrator → grants auto
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT SELECT ON TABLES TO app_readonly;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT ALL ON TABLES TO app_admin;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO app_user, app_admin;
