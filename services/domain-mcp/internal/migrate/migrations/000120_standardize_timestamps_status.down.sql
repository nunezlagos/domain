-- Rollback de 000120: remueve el trigger trg_set_updated_at de todas las tablas
-- operativas (espejo dinámico del up).
--
-- NO se dropea la función set_updated_at(): es compartida (definida en 000001 y
-- usada por otros triggers como set_updated_at_<tabla>); el up solo la hace
-- CREATE OR REPLACE. Dropearla rompería los demás rollbacks.
--
-- NO se borran columnas created_at/updated_at/status: pueden tener datos críticos
-- (mismo criterio conservador que tenía este down).

DO $$
DECLARE
  t record;
BEGIN
  FOR t IN
    SELECT table_name
    FROM information_schema.tables
    WHERE table_schema = 'public'
      AND table_type = 'BASE TABLE'
      AND table_name <> 'schema_migrations'
      AND table_name NOT LIKE 'pg_%'
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_set_updated_at ON %I', t.table_name);
  END LOOP;
END
$$;
