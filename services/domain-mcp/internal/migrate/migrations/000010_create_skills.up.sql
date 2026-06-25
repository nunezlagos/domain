






CREATE TABLE skills (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug VARCHAR(100) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  description_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', coalesce(description, ''))) STORED,
  skill_type VARCHAR(20) NOT NULL,
  content TEXT,
  input_schema JSONB NOT NULL DEFAULT '{}',
  output_schema JSONB NOT NULL DEFAULT '{}',
  timeout_seconds INT NOT NULL DEFAULT 30 CHECK (timeout_seconds BETWEEN 1 AND 600),
  idempotent BOOLEAN NOT NULL DEFAULT false,
  has_side_effects BOOLEAN NOT NULL DEFAULT false,
  depends_on TEXT[] NOT NULL DEFAULT '{}',
  tags TEXT[] NOT NULL DEFAULT '{}',
  embedding vector(1536),
  seed_managed BOOLEAN NOT NULL DEFAULT false,
  seed_version INT,
  is_user_modified BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, slug),
  CHECK (skill_type IN ('prompt', 'code', 'api', 'mcp_tool'))
);

CREATE TRIGGER set_updated_at_skills
  BEFORE UPDATE ON skills
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX skills_organization_idx ON skills (organization_id) WHERE deleted_at IS NULL;
CREATE INDEX skills_description_tsv_idx ON skills USING GIN (description_tsv);
CREATE INDEX skills_embedding_idx ON skills USING ivfflat (embedding vector_cosine_ops) WITH (lists = 50);
CREATE INDEX skills_tags_idx ON skills USING GIN (tags);
