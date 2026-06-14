-- migration: skills_add_project_id
-- author: mnunez@saargo.com
-- issue: REQ-44 skills por proyecto (Ola B)
-- description: agrega skills.project_id nullable. NULL = skill global de
--   la org; not-NULL = skill específica del proyecto. Resolver:
--   project_id match → fallback global (NULL). El UNIQUE (org, slug)
--   se reemplaza por UNIQUE (org, project_id, slug) para permitir mismo
--   slug en distintos scopes.
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE skills
  ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

-- Drop UNIQUE viejo si existe (puede tener distintos nombres según pg version).
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'skills_organization_id_slug_key'
  ) THEN
    ALTER TABLE skills DROP CONSTRAINT skills_organization_id_slug_key;
  END IF;
END$$;

-- UNIQUE diferenciado: para project_id NULL (globales), y para project_id NOT NULL.
-- Postgres trata NULL como distinto en UNIQUE indexes, así que necesitamos 2
-- partial indexes en lugar de UNIQUE constraint estándar.
CREATE UNIQUE INDEX IF NOT EXISTS skills_org_slug_global_uniq
  ON skills (organization_id, slug)
  WHERE project_id IS NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS skills_org_project_slug_uniq
  ON skills (organization_id, project_id, slug)
  WHERE project_id IS NOT NULL AND deleted_at IS NULL;

-- Index para resolver project → fallback global rápido.
CREATE INDEX IF NOT EXISTS skills_org_project_idx
  ON skills (organization_id, project_id)
  WHERE deleted_at IS NULL;
