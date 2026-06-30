






CREATE TABLE webhooks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  slug VARCHAR(100) NOT NULL,
  name VARCHAR(255) NOT NULL,
  secret_encrypted BYTEA NOT NULL,
  source_type VARCHAR(30) NOT NULL DEFAULT 'generic',
  target_type VARCHAR(20) NOT NULL,
  target_id UUID NOT NULL,
  inputs_mapping JSONB NOT NULL DEFAULT '{}',
  enabled BOOLEAN NOT NULL DEFAULT true,
  last_delivery_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, slug),
  CHECK (target_type IN ('flow', 'agent', 'skill')),
  CHECK (source_type IN ('generic', 'github', 'gitlab', 'bitbucket'))
);

CREATE TABLE webhook_deliveries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
  payload JSONB NOT NULL,
  headers JSONB NOT NULL DEFAULT '{}',
  source_ip VARCHAR(45),
  status VARCHAR(20) NOT NULL,
  error TEXT,
  triggered_run_id UUID,
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (status IN ('received', 'signature_invalid', 'mapped', 'triggered', 'failed'))
);

CREATE TRIGGER set_updated_at_webhooks
  BEFORE UPDATE ON webhooks
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX webhooks_slug_idx ON webhooks (organization_id, slug) WHERE enabled = true AND deleted_at IS NULL;
CREATE INDEX webhook_deliveries_webhook_idx ON webhook_deliveries (webhook_id, received_at DESC);
