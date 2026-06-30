













DO $$
DECLARE
    r RECORD;
    drop_count INT := 0;
BEGIN
    FOR r IN (
        SELECT conrelid::regclass::text AS tbl, conname
        FROM pg_constraint
        WHERE confrelid = 'organizations'::regclass
          AND contype = 'f'
        ORDER BY conrelid::regclass::text
    ) LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I', r.tbl, r.conname);
        drop_count := drop_count + 1;
        RAISE NOTICE 'dropped FK: %.%', r.tbl, r.conname;
    END LOOP;
    RAISE NOTICE 'total FKs dropped: %', drop_count;
END $$;
