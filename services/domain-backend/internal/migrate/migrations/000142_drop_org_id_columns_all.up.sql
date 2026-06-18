-- migration: drop_org_id_columns_all
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase C — destructiva, irreversible sin restore)
-- description: DROP COLUMN organization_id en TODAS las tablas que aún la
--   tengan en el schema public. Cubre las 49 tablas restantes después de
--   000141 (las 5 satélites per-consumer). Usa information_schema para
--   enumerar dinámicamente — seguro si una tabla fue dropeada en otra
--   migración (no falla con "column does not exist").
--   Pre-requisito: 000140 ejecutada (FKs a organizations(id) dropeadas).
--   Preserva filas (PG no toca data al dropear columnas no-PK). Cada tabla
--   mantiene su PK existente; organization_id desaparece completamente.
--   Tras esta migración:
--     - 0 columnas organization_id en el schema public
--     - 0 FKs hacia organizations(id)
--     - organizations sigue existiendo (vacía) — se dropea en 000143
-- breaking: true (datos organization_id se pierden — restore vía pgBackRest)
-- estimated_duration: <5s (54 ALTERs)

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
