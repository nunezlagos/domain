

















CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;





DO $$
DECLARE
  t record;
  has_created boolean;
  has_updated boolean;
  has_status boolean;
BEGIN
  FOR t IN
    SELECT table_name
    FROM information_schema.tables
    WHERE table_schema = 'public'
      AND table_type = 'BASE TABLE'
      AND table_name <> 'schema_migrations'
      AND table_name NOT LIKE 'pg_%'
  LOOP

    SELECT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = 'public'
        AND table_name = t.table_name
        AND column_name = 'created_at'
    ) INTO has_created;

    IF NOT has_created THEN
      EXECUTE format(
        'ALTER TABLE %I ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()',
        t.table_name
      );
    END IF;


    SELECT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = 'public'
        AND table_name = t.table_name
        AND column_name = 'updated_at'
    ) INTO has_updated;

    IF NOT has_updated THEN
      EXECUTE format(
        'ALTER TABLE %I ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()',
        t.table_name
      );
    END IF;


    SELECT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = 'public'
        AND table_name = t.table_name
        AND column_name = 'status'
    ) INTO has_status;

    IF NOT has_status THEN
      EXECUTE format(
        'ALTER TABLE %I ADD COLUMN status TEXT NOT NULL DEFAULT %L',
        t.table_name, 'active'
      );
    END IF;


    EXECUTE format(
      'DROP TRIGGER IF EXISTS trg_set_updated_at ON %I', t.table_name
    );
    EXECUTE format(
      'CREATE TRIGGER trg_set_updated_at BEFORE UPDATE ON %I
        FOR EACH ROW EXECUTE FUNCTION set_updated_at()',
      t.table_name
    );


    EXECUTE format(
      'CREATE INDEX IF NOT EXISTS %I_status_idx ON %I (status)',
      t.table_name, t.table_name
    );
  END LOOP;
END
$$;





COMMENT ON FUNCTION set_updated_at() IS
  'Trigger function: actualiza updated_at a NOW() en cada UPDATE.
   Aplicada a todas las tablas operativas via trg_set_updated_at.';


DO $$
BEGIN
  RAISE NOTICE 'Migración 000120: created_at, updated_at, status estandarizados en todas las tablas operativas';
END
$$;
