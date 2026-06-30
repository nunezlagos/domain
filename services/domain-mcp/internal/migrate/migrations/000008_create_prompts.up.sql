






CREATE TABLE prompts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  slug VARCHAR(100) NOT NULL,
  version INT NOT NULL DEFAULT 1,
  body TEXT NOT NULL,
  body_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', body)) STORED,
  variables JSONB NOT NULL DEFAULT '[]',
  description TEXT,
  is_active BOOLEAN NOT NULL DEFAULT true,
  parent_version_id UUID,
  tags TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, project_id, slug, version)
);

CREATE TRIGGER set_updated_at_prompts
  BEFORE UPDATE ON prompts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX prompts_project_active_idx ON prompts (project_id, slug)
  WHERE is_active = true AND deleted_at IS NULL;
CREATE INDEX prompts_body_tsv_idx ON prompts USING GIN (body_tsv);
