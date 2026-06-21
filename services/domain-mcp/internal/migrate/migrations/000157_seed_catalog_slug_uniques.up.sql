-- migration: seed_catalog_slug_uniques
-- description: recrea los índices únicos de `slug` que los seeders de catálogo
--   (skills, agent_templates, flows) necesitan para su `ON CONFLICT (slug)`.
--   Al eliminar organization_id (REQ-143) se cayeron los UNIQUE(organization_id,
--   slug) de esas tablas y no se recrearon sobre (slug) solo → el seed fallaba
--   con SQLSTATE 42P10 (no unique/exclusion constraint matching ON CONFLICT).
--   Las tablas son catálogos vacíos → sin riesgo de duplicados.
-- breaking: false

-- skills: el seeder hace ON CONFLICT (slug) WHERE project_id IS NULL AND deleted_at IS NULL
CREATE UNIQUE INDEX IF NOT EXISTS skills_slug_global_uniq
  ON skills (slug)
  WHERE project_id IS NULL AND deleted_at IS NULL;

-- agent_templates: el seeder hace ON CONFLICT (slug)
CREATE UNIQUE INDEX IF NOT EXISTS agent_templates_slug_uniq
  ON agent_templates (slug);

-- flows: el seeder hace ON CONFLICT (slug)
CREATE UNIQUE INDEX IF NOT EXISTS flows_slug_uniq
  ON flows (slug);
