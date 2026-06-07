-- migration: create_observations
-- author: mnunez@saargo.com
-- issue: HU-01.1 + HU-03.1
-- description: observations con vector(1536) embedding + content_tsv FTS GIN
-- breaking: false
-- estimated_duration: <1s (empty)

CREATE TABLE observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  session_id UUID,
  content TEXT NOT NULL,
  content_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', content)) STORED,
  embedding vector(1536),
  observation_type VARCHAR(50) NOT NULL DEFAULT 'note',
  tags TEXT[] NOT NULL DEFAULT '{}',
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE TRIGGER set_updated_at_observations
  BEFORE UPDATE ON observations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX observations_project_created_idx ON observations (project_id, created_at DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX observations_content_tsv_idx ON observations USING GIN (content_tsv);
CREATE INDEX observations_embedding_idx ON observations USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 100);
CREATE INDEX observations_tags_idx ON observations USING GIN (tags);
