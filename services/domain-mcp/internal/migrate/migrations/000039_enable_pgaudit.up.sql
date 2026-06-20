-- migration: enable_pgaudit
-- author: nunezlagos
-- issue: HU-25.7
-- description: habilita extension pgaudit + security labels para object audit
-- breaking: false
-- estimated_duration: <1s
--
-- IMPORTANTE: requiere que postgresql.conf tenga
--   shared_preload_libraries = 'pg_stat_statements,pgaudit'
-- y restart del cluster ANTES de aplicar esta migration.
-- Si pgaudit no está instalado en el cluster (testcontainers, dev images sin
-- la extension), la creación se omite vía NOTICE — la migration queda no-op
-- pero idempotente y los demás pasos del schema avanzan sin bloqueo.
--
-- squawk-ignore: ban-create-extension
-- reason: pgaudit es server-managed; idempotent con IF NOT EXISTS

DO $$
BEGIN
  BEGIN
    EXECUTE 'CREATE EXTENSION IF NOT EXISTS pgaudit';
  EXCEPTION
    WHEN feature_not_supported THEN
      RAISE NOTICE 'pgaudit not loaded via shared_preload_libraries; skipping';
    WHEN undefined_file THEN
      RAISE NOTICE 'pgaudit extension files not installed on cluster; skipping';
    WHEN OTHERS THEN
      RAISE NOTICE 'pgaudit installation skipped (%): %', SQLSTATE, SQLERRM;
  END;
END$$;

-- Object-level audit en tablas con datos altamente sensibles.
-- READ,WRITE → captura SELECT + INSERT/UPDATE/DELETE.
-- pgaudit ignora silenciosamente los labels si la extension no está loaded
-- (lo cual NO debería pasar en prod; en dev local sin pgaudit, no-op).
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pgaudit') THEN
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE audit_log IS ''READ,WRITE''';
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE api_keys IS ''READ,WRITE''';
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE plan_assignments IS ''READ,WRITE''';
  END IF;
END$$;
