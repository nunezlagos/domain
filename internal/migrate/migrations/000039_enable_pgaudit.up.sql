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
-- Sin esto, CREATE EXTENSION pgaudit fallará con
--   ERROR: pgaudit must be loaded via shared_preload_libraries.
--
-- squawk-ignore: ban-create-extension
-- reason: pgaudit es server-managed; idempotent con IF NOT EXISTS

CREATE EXTENSION IF NOT EXISTS pgaudit;

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
