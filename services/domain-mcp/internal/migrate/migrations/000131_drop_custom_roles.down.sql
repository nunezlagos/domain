
CREATE TABLE IF NOT EXISTS custom_roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug VARCHAR(50) NOT NULL,
  name VARCHAR(100) NOT NULL,
  permissions JSONB NOT NULL,
  description TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE(organization_id, slug)
);

CREATE INDEX IF NOT EXISTS custom_roles_org_idx ON custom_roles (organization_id);


CREATE OR REPLACE FUNCTION notify_custom_roles_changed() RETURNS trigger AS $$
BEGIN
  PERFORM pg_notify('custom_roles_changed', '');
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;



CREATE TRIGGER custom_roles_notify_mod
  AFTER INSERT OR UPDATE OR DELETE ON custom_roles
  FOR EACH STATEMENT EXECUTE FUNCTION notify_custom_roles_changed();
