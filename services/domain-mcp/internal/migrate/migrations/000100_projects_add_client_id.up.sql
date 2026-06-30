











ALTER TABLE projects
  ADD COLUMN client_id UUID REFERENCES clients(id) ON DELETE SET NULL;

CREATE INDEX projects_client_id_idx ON projects (client_id)
  WHERE deleted_at IS NULL AND client_id IS NOT NULL;


CREATE OR REPLACE FUNCTION projects_check_client_same_org() RETURNS TRIGGER AS $$
DECLARE
  client_org UUID;
BEGIN
  IF NEW.client_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT organization_id INTO client_org FROM clients WHERE id = NEW.client_id;
  IF client_org IS NULL THEN
    RAISE EXCEPTION 'client_id % does not exist', NEW.client_id
      USING ERRCODE = 'foreign_key_violation';
  END IF;
  IF client_org <> NEW.organization_id THEN
    RAISE EXCEPTION 'client.organization_id (%) must match project.organization_id (%)',
      client_org, NEW.organization_id
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER projects_client_same_org_check
  BEFORE INSERT OR UPDATE OF client_id, organization_id ON projects
  FOR EACH ROW EXECUTE FUNCTION projects_check_client_same_org();
