-- migration: tickets_external_unique
-- author: mnunez@saargo.com
-- issue: REQ-59 prevenir external_id duplicado
-- description: dos tickets distintos NO pueden linkear al mismo
--   external_id en un mismo (org, provider). Sin esta constraint el
--   webhook de Jira no sabría a qué ticket aplicar updates si dos
--   apuntan a MPS-12.
-- breaking: false
-- estimated_duration: <1s

CREATE UNIQUE INDEX IF NOT EXISTS project_tickets_external_unique
  ON project_tickets (organization_id, external_provider, external_id)
  WHERE external_id IS NOT NULL
    AND external_provider IS NOT NULL
    AND deleted_at IS NULL;
