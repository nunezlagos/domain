-- migration: create_event_log
-- author: nunezlagos
-- issue: HU-10.3
-- description: event log para event-execution bus
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE event_log (
  id UUID PRIMARY KEY,
  type VARCHAR(80) NOT NULL,
  organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
  project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  payload JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX event_log_type_created_idx ON event_log (type, created_at DESC);
CREATE INDEX event_log_org_created_idx ON event_log (organization_id, created_at DESC)
  WHERE organization_id IS NOT NULL;

GRANT SELECT, INSERT ON event_log TO app_user;
