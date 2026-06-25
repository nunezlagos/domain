









CREATE UNIQUE INDEX IF NOT EXISTS project_tickets_external_unique
  ON project_tickets (organization_id, external_provider, external_id)
  WHERE external_id IS NOT NULL
    AND external_provider IS NOT NULL
    AND deleted_at IS NULL;
