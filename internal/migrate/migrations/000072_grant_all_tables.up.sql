-- migration: grant_all_tables_to_app_roles
-- author: nunezlagos
-- issue: schema audit fix — tablas creadas como test (testcontainers) NO
--        heredaron los DEFAULT PRIVILEGES de app_migrator.
-- description: barre y aplica GRANTs explícitos a todas las tablas y
--              sequences existentes en public para app_user + app_admin.
-- breaking: false
-- estimated_duration: <1s
--
-- Fondo del bug:
--   * Migration 000025 sólo hace ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator.
--   * En tests + dev local las migrations corren como user `test` (no
--     app_migrator) → los default privileges no se gatillan.
--   * Resultado: app_user/app_admin no podían SELECT/INSERT contra tablas
--     creadas en migrations 26-71 que no incluyeron GRANT explícito.
--
-- Esta migration es idempotente: GRANT sobre tabla que ya tiene el grant
-- es no-op silencioso. Re-aplicar es seguro.

DO $grants$
DECLARE
  tbl text;
  seq text;
BEGIN
  -- Tablas: GRANT para app_user (CRUD) y app_admin (ALL).
  FOR tbl IN
    SELECT quote_ident(table_name)
    FROM information_schema.tables
    WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO app_user', tbl);
    EXECUTE format('GRANT ALL ON public.%s TO app_admin', tbl);
    EXECUTE format('GRANT SELECT ON public.%s TO app_readonly', tbl);
  END LOOP;

  -- Sequences: app_user + app_admin pueden usarlas.
  FOR seq IN
    SELECT quote_ident(sequence_name)
    FROM information_schema.sequences
    WHERE sequence_schema = 'public'
  LOOP
    EXECUTE format('GRANT USAGE, SELECT, UPDATE ON SEQUENCE public.%s TO app_user', seq);
    EXECUTE format('GRANT ALL ON SEQUENCE public.%s TO app_admin', seq);
  END LOOP;

  -- Views: solo SELECT.
  FOR tbl IN
    SELECT quote_ident(table_name)
    FROM information_schema.views
    WHERE table_schema = 'public'
  LOOP
    EXECUTE format('GRANT SELECT ON public.%s TO app_user', tbl);
    EXECUTE format('GRANT SELECT ON public.%s TO app_readonly', tbl);
    EXECUTE format('GRANT SELECT ON public.%s TO app_admin', tbl);
  END LOOP;
END
$grants$;

-- Future-proof: cualquier role que cree tablas futuras les aplica los
-- mismos grants por default. Esto solo funcionará para tablas creadas
-- POR el role current cuando esta migration corra; igualmente las
-- migrations futuras deberían incluir GRANT explícito.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT ALL ON TABLES TO app_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT ON TABLES TO app_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO app_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT ALL ON SEQUENCES TO app_admin;
