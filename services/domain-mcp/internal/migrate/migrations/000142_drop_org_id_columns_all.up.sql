

























DO $$
DECLARE
    r RECORD;
    drop_count INT := 0;
BEGIN

    FOR r IN (
        SELECT schemaname, tablename, policyname
        FROM pg_policies
        WHERE schemaname = 'public'
          AND (qual ILIKE '%organization_id%' OR with_check ILIKE '%organization_id%')
    ) LOOP
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I.%I',
                       r.policyname, r.schemaname, r.tablename);
        drop_count := drop_count + 1;
    END LOOP;


    FOR r IN (
        SELECT n.nspname AS schemaname, c.relname AS tablename, con.conname
        FROM pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND con.contype IN ('u', 'f', 'c', 'p')
          AND pg_get_constraintdef(con.oid) ILIKE '%organization_id%'
    ) LOOP
        EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS %I',
                       r.schemaname, r.tablename, r.conname);
        drop_count := drop_count + 1;
    END LOOP;


    FOR r IN (
        SELECT schemaname, indexname
        FROM pg_indexes
        WHERE schemaname = 'public'
          AND indexdef ILIKE '%organization_id%'
    ) LOOP
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', r.schemaname, r.indexname);
        drop_count := drop_count + 1;
    END LOOP;


    FOR r IN (
        SELECT n.nspname AS schemaname, c.relname AS tablename, t.tgname
        FROM pg_trigger t
        JOIN pg_class c ON c.oid = t.tgrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND NOT t.tgisinternal
          AND pg_get_triggerdef(t.oid) ILIKE '%organization_id%'
    ) LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I.%I',
                       r.tgname, r.schemaname, r.tablename);
        drop_count := drop_count + 1;
    END LOOP;




    FOR r IN (
        SELECT schemaname, viewname
        FROM pg_views
        WHERE schemaname = 'public'
          AND definition ILIKE '%organization_id%'
    ) LOOP
        EXECUTE format('DROP VIEW IF EXISTS %I.%I CASCADE',
                       r.schemaname, r.viewname);
        drop_count := drop_count + 1;
    END LOOP;

    RAISE NOTICE 'pre-cleanup: dropped % dependent objects', drop_count;
END $$;




DO $$
DECLARE
    r RECORD;
    drop_count INT := 0;
BEGIN
    FOR r IN (
        SELECT c.table_schema, c.table_name
        FROM information_schema.columns c
        WHERE c.column_name = 'organization_id'
          AND c.table_schema = 'public'
        ORDER BY c.table_name
    ) LOOP
        EXECUTE format('ALTER TABLE %I.%I DROP COLUMN IF EXISTS organization_id',
                       r.table_schema, r.table_name);
        drop_count := drop_count + 1;
        RAISE NOTICE 'dropped organization_id from: %.%', r.table_schema, r.table_name;
    END LOOP;
    RAISE NOTICE 'total columns dropped: %', drop_count;
END $$;
