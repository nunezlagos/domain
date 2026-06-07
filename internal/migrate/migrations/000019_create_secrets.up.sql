-- migration: create_secrets
-- author: mnunez@saargo.com
-- issue: HU-01.1 + HU-02.3
-- description: secrets cifrados AES-256-GCM (master key gestionada via KMS/Vault)
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE secrets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug VARCHAR(100) NOT NULL,
  name VARCHAR(255) NOT NULL,
  encrypted_value BYTEA NOT NULL,
  encryption_key_version INT NOT NULL DEFAULT 1,
  description TEXT,
  expires_at TIMESTAMPTZ,
  rotated_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, slug)
);

CREATE TRIGGER set_updated_at_secrets
  BEFORE UPDATE ON secrets
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX secrets_organization_idx ON secrets (organization_id) WHERE deleted_at IS NULL;
