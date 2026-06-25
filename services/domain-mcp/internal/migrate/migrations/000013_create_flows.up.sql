






CREATE TABLE flows (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug VARCHAR(100) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  spec JSONB NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  deterministic_replay BOOLEAN NOT NULL DEFAULT false,
  seed_managed BOOLEAN NOT NULL DEFAULT false,
  seed_version INT,
  is_user_modified BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, slug)
);

CREATE TRIGGER set_updated_at_flows
  BEFORE UPDATE ON flows
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX flows_organization_idx ON flows (organization_id) WHERE deleted_at IS NULL;
