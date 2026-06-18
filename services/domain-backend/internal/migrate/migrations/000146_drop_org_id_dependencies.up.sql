-- migration: drop_org_id_dependencies
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase C — pre-cleanup)
-- description: drop_organization_fks (000140) dropea las FKs a organizations(id),
--   pero NO dropea policies RLS / índices / constraints / triggers que
--   referencian la columna organization_id de cada tabla. La 000142 hace
--   DROP COLUMN organization_id directamente y FALLA con "cannot drop column
--   X of table Y because other objects depend on it" si no se dropean antes.
--   Esta migration pre-cleanup dropea TODO lo que pueda depender de la
--   columna organization_id antes de la 142. Pre-requisito: 000140 ejecutada.
-- breaking: false (solo dropea objects derivados de la columna)
-- estimated_duration: <1s

DO $$
DECLARE
    r RECORD;
    drop_count INT := 0;
BEGIN
    -- 1) Policies RLS que mencionan organization_id en qual o with_check.
    FOR r IN (
        SELECT schemaname, tablename, policyname
        FROM pg_policies
        WHERE schemaname = 'public'
          AND (qual ILIKE '%organization_id%' OR with_check ILIKE '%organization_id%')
    ) LOOP
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I.%I',
                       r.policyname, r.schemaname, r.tablename);
        drop_count := drop_count + 1;
        RAISE NOTICE 'dropped policy: %.%.%', r.schemaname, r.tablename, r.policyname;
    END LOOP;

    -- 2) Constraints (UNIQUE, FK, CHECK, PK) que mencionan organization_id.
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
        RAISE NOTICE 'dropped constraint: %.%.%', r.schemaname, r.tablename, r.conname;
    END LOOP;

    -- 3) Índices que mencionan organization_id (los huérfanos que no son
    --    constraints ya fueron dropeados en el paso 2).
    FOR r IN (
        SELECT schemaname, indexname
        FROM pg_indexes
        WHERE schemaname = 'public'
          AND indexdef ILIKE '%organization_id%'
    ) LOOP
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', r.schemaname, r.indexname);
        drop_count := drop_count + 1;
        RAISE NOTICE 'dropped index: %.%', r.schemaname, r.indexname;
    END LOOP;

    -- 4) Triggers que mencionan organization_id en su definición.
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
        RAISE NOTICE 'dropped trigger: %.%.%', r.schemaname, r.tablename, r.tgname;
    END LOOP;

    RAISE NOTICE 'total objects dropped: %', drop_count;
END $$;
