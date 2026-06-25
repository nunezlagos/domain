






CREATE TABLE activity_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action VARCHAR(100) NOT NULL,
  entity_type VARCHAR(100) NOT NULL,
  entity_id UUID,
  summary TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}',
  visibility VARCHAR(20) NOT NULL DEFAULT 'org',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (visibility IN ('public', 'org', 'project', 'private'))
);

CREATE INDEX activity_log_org_created_idx ON activity_log (organization_id, created_at DESC);
CREATE INDEX activity_log_project_idx ON activity_log (project_id, created_at DESC)
  WHERE project_id IS NOT NULL;
CREATE INDEX activity_log_actor_idx ON activity_log (actor_id, created_at DESC)
  WHERE actor_id IS NOT NULL;
CREATE INDEX activity_log_entity_idx ON activity_log (entity_type, entity_id);
