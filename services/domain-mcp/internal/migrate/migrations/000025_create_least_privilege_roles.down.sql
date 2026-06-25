

DO $$
DECLARE
  db_name TEXT := current_database();
BEGIN

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


  DROP ROLE IF EXISTS app_user;
  DROP ROLE IF EXISTS app_admin;
  DROP ROLE IF EXISTS app_readonly;
  DROP ROLE IF EXISTS app_migrator;
END $$;


GRANT CREATE ON SCHEMA public TO PUBLIC;
