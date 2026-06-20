-- migration: proposals_proposed_flag
-- author: mnunez@saargo.com
-- issue: REQ-49 proposals de policies/skills auto-generadas (Ola E)
-- description: agrega proposed BOOL a project_policies y skills. Cuando
--   un LLM crea con source='llm_generated' opcionalmente lo marca como
--   proposed=true → la entry queda invisible para los resolvers (policy
--   _get, skill_search/list) hasta que el usuario la aprueba con
--   domain_proposal_review. proposed=false (default) = visible y
--   funcional inmediatamente.
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE project_policies
  ADD COLUMN IF NOT EXISTS proposed BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE skills
  ADD COLUMN IF NOT EXISTS proposed BOOLEAN NOT NULL DEFAULT false;

-- Updates a los indexes ya existentes para que los proposed NO aparezcan
-- en queries de "lo activo" — el resolver natural los ignora.
DROP INDEX IF EXISTS project_policies_org_project_idx;
CREATE INDEX project_policies_org_project_idx
  ON project_policies (organization_id, project_id)
  WHERE deleted_at IS NULL AND is_active = TRUE AND proposed = false;

DROP INDEX IF EXISTS skills_organization_idx;
CREATE INDEX skills_organization_idx
  ON skills (organization_id)
  WHERE deleted_at IS NULL AND proposed = false;

-- Index para listar proposals pendientes rápido
CREATE INDEX IF NOT EXISTS project_policies_proposed_idx
  ON project_policies (organization_id, project_id, created_at DESC)
  WHERE proposed = true AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS skills_proposed_idx
  ON skills (organization_id, created_at DESC)
  WHERE proposed = true AND deleted_at IS NULL;
