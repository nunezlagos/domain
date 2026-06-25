









CREATE INDEX IF NOT EXISTS project_tickets_due_date_idx
  ON project_tickets (organization_id, due_date)
  WHERE due_date IS NOT NULL
    AND status NOT IN ('done','cancelled')
    AND deleted_at IS NULL;


CREATE INDEX IF NOT EXISTS project_tickets_client_idx
  ON project_tickets (organization_id, client_id, status, updated_at DESC)
  WHERE client_id IS NOT NULL AND deleted_at IS NULL;
