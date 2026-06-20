-- migration: tickets_extra_indexes
-- author: mnunez@saargo.com
-- issue: REQ-61 indexes faltantes en project_tickets
-- description: cobertura para queries comunes que hoy caen en seq scan:
--   - tickets con due_date próximo (alertas / cronjobs)
--   - tickets por cliente/mandante (consultoras con varios clientes)
-- breaking: false
-- estimated_duration: <1s

-- Tickets activos con due_date — para "qué vence pronto"
CREATE INDEX IF NOT EXISTS project_tickets_due_date_idx
  ON project_tickets (organization_id, due_date)
  WHERE due_date IS NOT NULL
    AND status NOT IN ('done','cancelled')
    AND deleted_at IS NULL;

-- Tickets por cliente — listing en consultoras gestionando mandantes
CREATE INDEX IF NOT EXISTS project_tickets_client_idx
  ON project_tickets (organization_id, client_id, status, updated_at DESC)
  WHERE client_id IS NOT NULL AND deleted_at IS NULL;
