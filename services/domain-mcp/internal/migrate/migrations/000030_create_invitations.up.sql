






CREATE TABLE invitations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  invited_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  email VARCHAR(255) NOT NULL,
  role VARCHAR(50) NOT NULL DEFAULT 'member'
    CHECK (role IN ('admin','maintainer','member','viewer')),
  token UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
  status VARCHAR(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','accepted','declined','expired','revoked')),
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
  accepted_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE TRIGGER set_updated_at_invitations
  BEFORE UPDATE ON invitations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


CREATE INDEX invitations_org_idx ON invitations (organization_id, created_at DESC)
  WHERE deleted_at IS NULL;


CREATE INDEX invitations_email_idx ON invitations (email)
  WHERE status = 'pending';


CREATE UNIQUE INDEX invitations_org_email_pending_uniq
  ON invitations (organization_id, email)
  WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON invitations TO app_user;
GRANT ALL ON invitations TO app_admin;
