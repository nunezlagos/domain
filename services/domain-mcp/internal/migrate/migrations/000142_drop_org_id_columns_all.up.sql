-- migration: drop_org_id_columns_all
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase C — destructiva, irreversible sin restore)
-- description: DROP COLUMN organization_id en TODAS las tablas que aún la
--   tengan en el schema public. Cubre las 49 tablas restantes después de
--   000141 (las 5 satélites per-consumer).
--   ANTES de dropear las columnas, dropea todas las dependencies que
--   podrían bloquear el ALTER TABLE: policies RLS, constraints (UNIQUE/FK/
--   CHECK/PK), índices huérfanos y triggers no-internal. Esto es necesario
--   porque las migrations de RLS (000130-000139) crearon policies que
--   referencian organization_id y DROPEAR la columna sin dropear antes
--   las policies falla con SQLSTATE 2BP01 ("cannot drop column because
--   other objects depend on it").
--   Tras esta migración:
--     - 0 columnas organization_id en el schema public
--     - 0 FKs hacia organizations(id)
--     - 0 policies RLS sobre organization_id (el resto de RLS se mantiene)
--     - 0 índices sobre organization_id
--     - 0 constraints que mencionen organization_id
--     - organizations sigue existiendo (vacía) — se dropea en 000143
-- breaking: true (datos organization_id se pierden — restore vía pgBackRest)
-- estimated_duration: <5s (54 ALTERs + pre-cleanup)

-- =====================================================================
-- FASE 1: Pre-cleanup — dropear todas las dependencies de organization_id
-- =====================================================================
DO $$
DECLARE
    r RECORD;
    drop_count INT := 0;
BEGIN
    -- 1) Policies RLS que mencionan organization_id.
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
    END LOOP;

    -- 3) Índices huérfanos sobre organization_id (los que no son constraints).
    FOR r IN (
        SELECT schemaname, indexname
        FROM pg_indexes
        WHERE schemaname = 'public'
          AND indexdef ILIKE '%organization_id%'
    ) LOOP
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', r.schemaname, r.indexname);
        drop_count := drop_count + 1;
    END LOOP;

    -- 4) Triggers no-internal que mencionan organization_id.
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

    -- 5) VIEWS que mencionan organization_id en su definición. CASCADE
    --    remueve dependencias en cascada (e.g. domain_cost_daily_by_org
    --    depende de agent_runs.organization_id).
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

-- =====================================================================
-- FASE 2: DROP COLUMN organization_id de TODAS las tablas
-- =====================================================================
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
