-- migration: create_sessions
-- author: mnunez@saargo.com
-- issue: HU-01.1 + HU-03.2
-- description: sesiones de trabajo agrupan observations
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title VARCHAR(500),
  summary TEXT,
  summary_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', coalesce(summary, ''))) STORED,
  tags TEXT[] NOT NULL DEFAULT '{}',
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ended_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE TRIGGER set_updated_at_sessions
  BEFORE UPDATE ON sessions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX sessions_user_started_idx ON sessions (user_id, started_at DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX sessions_project_idx ON sessions (project_id) WHERE deleted_at IS NULL;
CREATE INDEX sessions_summary_tsv_idx ON sessions USING GIN (summary_tsv);
