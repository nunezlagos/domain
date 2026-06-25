










CREATE TABLE verifications (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_id      UUID REFERENCES sessions(id) ON DELETE SET NULL,
  kind            VARCHAR(40) NOT NULL
    CHECK (kind IN ('build','test','lint','smoke','typecheck','migration','custom')),



  items           JSONB NOT NULL DEFAULT '[]',
  status          VARCHAR(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','running','passed','failed','partial')),
  context         TEXT,  -- qué cambio gatilló esta verificación
  started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX verifications_org_project_status_idx
  ON verifications (organization_id, project_id, status, started_at DESC);
CREATE INDEX verifications_org_session_idx
  ON verifications (organization_id, session_id) WHERE session_id IS NOT NULL;
CREATE INDEX verifications_pending_idx
  ON verifications (organization_id, status, started_at DESC)
  WHERE status IN ('pending','running');

ALTER TABLE verifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE verifications FORCE ROW LEVEL SECURITY;
CREATE POLICY verifications_org_isolation ON verifications
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON verifications TO app_user;
GRANT ALL ON verifications TO app_admin;
