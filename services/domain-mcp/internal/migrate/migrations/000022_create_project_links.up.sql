






CREATE TABLE project_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  linked_project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  access_level VARCHAR(20) NOT NULL DEFAULT 'read',
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (project_id, linked_project_id),
  CHECK (access_level IN ('read', 'write')),
  CHECK (project_id != linked_project_id)
);

CREATE INDEX project_links_project_idx ON project_links (project_id);
CREATE INDEX project_links_linked_idx ON project_links (linked_project_id);
