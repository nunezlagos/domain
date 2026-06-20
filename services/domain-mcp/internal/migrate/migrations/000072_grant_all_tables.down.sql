-- Down: revoca las DEFAULT PRIVILEGES seteadas por el up (sin FOR ROLE → pertenecen
-- al rol que corre las migraciones). Es necesario para que el down de 000025 pueda
-- DROP ROLE app_user/app_admin/app_readonly sin dependencias colgando.
--
-- NO se revierten los GRANT explícitos sobre tablas/secuencias existentes (sería
-- destructivo para apps en uso); solo las default privileges forward-looking.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
    ALTER DEFAULT PRIVILEGES IN SCHEMA public
      REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM app_user;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public
      REVOKE USAGE, SELECT, UPDATE ON SEQUENCES FROM app_user;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_admin') THEN
    ALTER DEFAULT PRIVILEGES IN SCHEMA public
      REVOKE ALL ON TABLES FROM app_admin;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public
      REVOKE ALL ON SEQUENCES FROM app_admin;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_readonly') THEN
    ALTER DEFAULT PRIVILEGES IN SCHEMA public
      REVOKE SELECT ON TABLES FROM app_readonly;
  END IF;
END $$;
