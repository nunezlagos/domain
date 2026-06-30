






CREATE TABLE IF NOT EXISTS agent_versions (
  id BIGSERIAL PRIMARY KEY,
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  version INT NOT NULL,
  snapshot JSONB NOT NULL DEFAULT '{}',
  changed_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (agent_id, version)
);



CREATE INDEX IF NOT EXISTS agent_versions_agent_idx
  ON agent_versions (agent_id, version DESC);

GRANT SELECT, INSERT, DELETE ON agent_versions TO app_user;
GRANT USAGE ON SEQUENCE agent_versions_id_seq TO app_user;
GRANT ALL ON agent_versions TO app_admin;
GRANT ALL ON SEQUENCE agent_versions_id_seq TO app_admin;
