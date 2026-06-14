-- migration: create_external_sync_state
-- author: nunezlagos
-- issue: HU-04.9
-- description: mirror state de REQs/HUs en providers externos (Jira MVP)
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE external_providers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  provider VARCHAR(30) NOT NULL
    CHECK (provider IN ('jira','github','linear','asana')),
  display_name VARCHAR(120) NOT NULL,
  base_url TEXT NOT NULL,
  project_key VARCHAR(60),                 -- Jira project key / GH repo / etc
  config JSONB NOT NULL DEFAULT '{}',      -- auth ref, field_mappings, status_mapping
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT external_providers_org_provider_unique UNIQUE (organization_id, provider, project_key)
);

CREATE TRIGGER set_updated_at_external_providers
  BEFORE UPDATE ON external_providers
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE external_sync_state (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_id UUID NOT NULL REFERENCES external_providers(id) ON DELETE CASCADE,
  entity_kind VARCHAR(20) NOT NULL
    CHECK (entity_kind IN ('req','hu')),
  entity_id UUID NOT NULL,
  external_key VARCHAR(60) NOT NULL,        -- DIDE-100, GH#42
  external_url TEXT NOT NULL,
  external_type VARCHAR(30),                -- Epic | Story | Issue
  sync_direction VARCHAR(20) NOT NULL DEFAULT 'push_only'
    CHECK (sync_direction IN ('push_only','pull_only','bidirectional')),
  sync_status VARCHAR(20) NOT NULL DEFAULT 'pending'
    CHECK (sync_status IN ('pending','ok','partial','conflict','disabled','failed')),
  field_mapping JSONB NOT NULL DEFAULT '{}',
  last_pushed_at TIMESTAMPTZ,
  last_pulled_at TIMESTAMPTZ,
  last_synced_at TIMESTAMPTZ,
  drift_detected_at TIMESTAMPTZ,
  drift_fields JSONB,
  partial_failures JSONB,
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT external_sync_state_provider_entity_unique
    UNIQUE (provider_id, entity_kind, entity_id),
  CONSTRAINT external_sync_state_provider_key_unique
    UNIQUE (provider_id, external_key)
);

CREATE INDEX external_sync_state_entity_idx
  ON external_sync_state (entity_kind, entity_id);
CREATE INDEX external_sync_state_status_idx
  ON external_sync_state (sync_status, next_retry_at)
  WHERE sync_status IN ('pending','conflict','failed');

CREATE TRIGGER set_updated_at_external_sync_state
  BEFORE UPDATE ON external_sync_state
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE external_sync_events (
  id BIGSERIAL PRIMARY KEY,
  sync_state_id UUID NOT NULL REFERENCES external_sync_state(id) ON DELETE CASCADE,
  event_type VARCHAR(30) NOT NULL,         -- push.ok, push.partial, pull.status, drift.detected, ...
  direction VARCHAR(10) NOT NULL CHECK (direction IN ('push','pull')),
  payload JSONB NOT NULL DEFAULT '{}',
  error_message TEXT,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX external_sync_events_state_idx
  ON external_sync_events (sync_state_id, occurred_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON external_providers TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON external_sync_state TO app_user;
GRANT SELECT, INSERT ON external_sync_events TO app_user;
GRANT USAGE, SELECT ON SEQUENCE external_sync_events_id_seq TO app_user;
