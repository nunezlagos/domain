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
--   Reversible: el down hace SET NOT NULL (falla si hay filas NULL) +
--   recrea los UNIQUE constraints. roundtrip (DB fresh) es seguro.
--
--   Pre-requisito: 000135-000139 aplicadas (cleanup parcial de Fase B).
-- breaking: false (no cambia schema funcional — solo permite NULL)
-- estimated_duration: <1s

BEGIN;

-- 1. Drop NOT NULL en las 6 columnas.
ALTER TABLE external_providers ALTER COLUMN organization_id DROP NOT NULL;
ALTER TABLE event_log ALTER COLUMN organization_id DROP NOT NULL;
ALTER TABLE projects ALTER COLUMN organization_id DROP NOT NULL;
ALTER TABLE intake_payloads ALTER COLUMN organization_id DROP NOT NULL;
ALTER TABLE selfhosted_runners ALTER COLUMN organization_id DROP NOT NULL;
ALTER TABLE selfhosted_tasks ALTER COLUMN organization_id DROP NOT NULL;

-- 2. Drop UNIQUE constraints que incluían organization_id (single-org
--    no aplica scope por org; los callers deben garantizar unicidad
--    via app, no via DB).
ALTER TABLE external_providers
  DROP CONSTRAINT IF EXISTS external_providers_org_provider_unique;

-- selfhosted_runners: UNIQUE(organization_id, name) en línea 16 de 000069.
-- Sin nombre explícito → Postgres usa <table>_<col1>_<col2>_key.
ALTER TABLE selfhosted_runners
  DROP CONSTRAINT IF EXISTS selfhosted_runners_organization_id_name_key;

-- 3. projects tiene UNIQUE (organization_id, slug). En single-org, el
--    caller garantiza slug único via validación previa (issue-05.5). El
--    UNIQUE index a nivel DB sobre slug solo (no (org, slug)) se mantiene
--    para defensa en profundidad.
ALTER TABLE projects
  DROP CONSTRAINT IF EXISTS projects_organization_id_slug_key;

COMMIT;
