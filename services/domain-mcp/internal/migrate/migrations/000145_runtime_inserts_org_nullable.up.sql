-- migration: runtime_inserts_org_nullable
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase D clean Round 3)
-- description: Hace nullable `organization_id` en las 6 tablas donde el
--   código de runtime hace INSERT con organization_id. Esto permite
--   refactorizar los INSERTs para NO incluir organization_id (single-org).
--
--   Tablas afectadas (todas tienen la columna NOT NULL con FK a organizations):
--   - external_providers (UNIQUE compuesto organization_id, provider, project_key)
--   - event_log (sin UNIQUE problemático — solo FK)
--   - projects (UNIQUE organization_id, slug)
--   - intake_payloads (sin UNIQUE problemático — solo FK)
--   - selfhosted_runners (sin UNIQUE problemático — solo FK)
--   - selfhosted_tasks (sin UNIQUE problemático — solo FK)
--
--   Cambios:
--   1. ALTER COLUMN DROP NOT NULL en las 6 tablas.
--   2. DROP CONSTRAINT external_providers_org_provider_unique (UNIQUE
--      compuesto que incluía organization_id — en single-org no aplica
--      el scope por org; el caller debe garantir unicidad via app).
--   3. DROP CONSTRAINT projects_org_slug_unique (mismo rationale).
--
--   IMPORTANTE: si la migration 000142 (drop_org_id_columns_all) ya corrió,
--   las columnas organization_id NO EXISTEN. En ese caso, esta migration
--   es NO-OP — todos los ALTER y DROP CONSTRAINT se skipean silenciosamente.
--   Esto permite que esta migration sea idempotente y reusable en installs
--   fresh (donde 000145 corre DESPUÉS de 000142).
--
--   Reversible: el down hace SET NOT NULL (falla si hay filas NULL) +
--   recrea los UNIQUE constraints. roundtrip (DB fresh) es seguro.
--
--   Pre-requisito: 000135-000139 aplicadas (cleanup parcial de Fase B).
-- breaking: false (no cambia schema funcional — solo permite NULL)
-- estimated_duration: <1s

BEGIN;

-- 1. Drop NOT NULL en las 6 columnas. Idempotente: si la columna no
--    existe (post-000142), el ALTER se skipea.
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'external_providers', 'event_log', 'projects',
        'intake_payloads', 'selfhosted_runners', 'selfhosted_tasks'
    ] LOOP
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema='public' AND table_name=t AND column_name='organization_id'
        ) THEN
            EXECUTE format('ALTER TABLE %I ALTER COLUMN organization_id DROP NOT NULL', t);
            RAISE NOTICE 'dropped NOT NULL on %.organization_id', t;
        ELSE
            RAISE NOTICE 'skipped (no column) %.organization_id', t;
        END IF;
    END LOOP;
END $$;

-- 2. Drop UNIQUE constraints que incluían organization_id. Idempotente:
--    IF EXISTS cubre el caso donde el constraint ya fue dropeado por
--    otra migration (e.g. 000142 CASCADE).
ALTER TABLE external_providers
  DROP CONSTRAINT IF EXISTS external_providers_org_provider_unique;

ALTER TABLE selfhosted_runners
  DROP CONSTRAINT IF EXISTS selfhosted_runners_organization_id_name_key;

ALTER TABLE projects
  DROP CONSTRAINT IF EXISTS projects_organization_id_slug_key;

COMMIT;
