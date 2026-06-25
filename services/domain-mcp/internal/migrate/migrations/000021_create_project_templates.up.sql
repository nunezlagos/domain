






CREATE TABLE project_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  slug VARCHAR(100) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  is_default BOOLEAN NOT NULL DEFAULT false,
  is_public BOOLEAN NOT NULL DEFAULT false,
  settings JSONB NOT NULL DEFAULT '{}',
  default_skills TEXT[] NOT NULL DEFAULT '{}',
  default_agents TEXT[] NOT NULL DEFAULT '{}',
  default_flows TEXT[] NOT NULL DEFAULT '{}',
  seed_managed BOOLEAN NOT NULL DEFAULT false,
  seed_version INT,
  is_user_modified BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (organization_id, slug)
);

CREATE TRIGGER set_updated_at_project_templates
  BEFORE UPDATE ON project_templates
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


ALTER TABLE projects
  ADD CONSTRAINT projects_template_id_fkey
  FOREIGN KEY (template_id) REFERENCES project_templates(id) ON DELETE SET NULL;
