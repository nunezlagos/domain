-- migration: drop_organization_fks
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase C — destructiva, irreversible sin restore)
-- description: dropea TODAS las foreign keys que apuntan a organizations(id)
--   desde 49 tablas. Pre-requisito absoluto para DROP TABLE organizations en
--   la migración 000143 (Fase C final). Sin este paso, el DROP TABLE falla
--   por dependencias FK. Esta migración es reversible (las FKs se recrean en
--   el down con sus acciones ON DELETE originales — RESTRICT para la mayoría,
--   SET NULL para algunas, CASCADE para pocas).
--   NO toca la columna organization_id (eso es 000141/000142).
--   NO dropea la tabla organizations (eso es 000143).
-- breaking: true
-- estimated_duration: <1s

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
