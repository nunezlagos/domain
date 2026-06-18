-- migration: drop_org_table_and_helpers
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase C — destructiva, irreversible sin restore)
-- description: DESTRUCTIVO FINAL. Dropea:
--   - Función current_org_id() — la GUC app.current_org_id deja de tener
--     significado (ya nadie la setea desde Fase A).
--   - Trigger projects_client_same_org_check — defense-in-depth obsoleto en
--     single-org (no hay cross-org que defender).
--   - Tabla organizations — root multi-tenant. Sus FKs ya están dropeadas
--     (000140) y organization_id ya no aparece en el schema (000141+142).
--     CASCADE remueve policies/triggers dependientes.
--   - organization_id UNIQUE INDEX en projects (huérfano tras el DROP).
--   - Columna organizations.plan_id (FK huérfana a plans, ya no se usa
--     para facturación — plans es interno de control de uso).
-- Pre-requisito: 000142 ejecutada (organization_id ya no existe en ninguna tabla).
-- breaking: true (organizations y sus datos se pierden — restore vía pgBackRest)
-- estimated_duration: <1s

-- Drop helper function. CASCADE remueve dependencias (triggers, views).
DROP FUNCTION IF EXISTS current_org_id() CASCADE;

-- Drop el trigger obsoleto (si existe)
DROP TRIGGER IF EXISTS projects_client_same_org_check ON projects;

-- Drop organizations.plan_id FK (la columna queda nullable — en este punto
-- es decisión del operador si la dropea con la tabla o la mantiene como
-- metadata. Como el código de billing NO usa organizations.plan_id ya
-- (REQ-21.5 single-org), la dropeamos junto con la tabla).
-- NOTA: ALTER en organizations antes del DROP TABLE.
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_plan_id_fkey;

-- Drop la tabla. CASCADE remueve indexes/triggers/policies dependientes.
DROP TABLE IF EXISTS organizations CASCADE;

-- Drop el UNIQUE INDEX huérfano sobre projects.organization_id (si quedó).
-- (Normalmente desapareció con la columna en 000142, pero si quedó algún
-- index que la referenciaba, lo limpiamos acá.)
DROP INDEX IF EXISTS projects_organization_id_unique;
