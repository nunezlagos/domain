















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


REVOKE CREATE ON SCHEMA public FROM PUBLIC;


DO $$
BEGIN
  EXECUTE format('REVOKE ALL ON DATABASE %I FROM PUBLIC', current_database());
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO app_user, app_admin, app_readonly, app_migrator', current_database());
END $$;

GRANT USAGE ON SCHEMA public TO app_user, app_admin, app_readonly;
GRANT USAGE, CREATE ON SCHEMA public TO app_migrator;


GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
REVOKE UPDATE, DELETE ON audit_log FROM app_user;


GRANT ALL ON ALL TABLES IN SCHEMA public TO app_admin;


GRANT SELECT ON ALL TABLES IN SCHEMA public TO app_readonly;


GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user, app_admin;


ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT SELECT ON TABLES TO app_readonly;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT ALL ON TABLES TO app_admin;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO app_user, app_admin;
