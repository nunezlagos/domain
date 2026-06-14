-- HU-25.6 down: limpia DEFAULT PRIVILEGES primero, después grants, después DROP roles.

DO $$
DECLARE
  db_name TEXT := current_database();
BEGIN
  -- 1. Limpiar DEFAULT PRIVILEGES (necesario antes de DROP ROLE)
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_migrator') THEN
    ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
      REVOKE ALL ON TABLES FROM app_user;
    ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
      REVOKE ALL ON TABLES FROM app_readonly;
    ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
      REVOKE ALL ON TABLES FROM app_admin;
    ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
      REVOKE ALL ON SEQUENCES FROM app_user;
    ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
      REVOKE ALL ON SEQUENCES FROM app_admin;
  END IF;

  -- 2. Revocar grants explícitos sobre objetos existentes
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_user') THEN
    REVOKE ALL ON ALL TABLES IN SCHEMA public FROM app_user;
    REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM app_user;
    REVOKE ALL ON SCHEMA public FROM app_user;
    EXECUTE format('REVOKE ALL ON DATABASE %I FROM app_user', db_name);
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_admin') THEN
    REVOKE ALL ON ALL TABLES IN SCHEMA public FROM app_admin;
    REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM app_admin;
    REVOKE ALL ON SCHEMA public FROM app_admin;
    EXECUTE format('REVOKE ALL ON DATABASE %I FROM app_admin', db_name);
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_migrator') THEN
    REVOKE ALL ON ALL TABLES IN SCHEMA public FROM app_migrator;
    REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM app_migrator;
    REVOKE ALL ON SCHEMA public FROM app_migrator;
    EXECUTE format('REVOKE ALL ON DATABASE %I FROM app_migrator', db_name);
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='app_readonly') THEN
    REVOKE ALL ON ALL TABLES IN SCHEMA public FROM app_readonly;
    REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM app_readonly;
    REVOKE ALL ON SCHEMA public FROM app_readonly;
    EXECUTE format('REVOKE ALL ON DATABASE %I FROM app_readonly', db_name);
  END IF;

  -- 3. DROP ROLE (order: dependent first)
  DROP ROLE IF EXISTS app_user;
  DROP ROLE IF EXISTS app_admin;
  DROP ROLE IF EXISTS app_readonly;
  DROP ROLE IF EXISTS app_migrator;
END $$;

-- Restaurar default público
GRANT CREATE ON SCHEMA public TO PUBLIC;
