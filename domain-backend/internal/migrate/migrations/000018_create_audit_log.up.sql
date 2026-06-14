-- migration: create_audit_log
-- author: mnunez@saargo.com
-- issue: HU-01.1 + HU-02.4 + RFC 0003
-- description: audit_log append-only inmutable (BIGSERIAL high volume)
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE audit_log (
  id BIGSERIAL PRIMARY KEY,
  organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
  actor_id UUID,
  actor_type VARCHAR(20) NOT NULL DEFAULT 'user',
  action VARCHAR(100) NOT NULL,
  entity_type VARCHAR(100) NOT NULL,
  entity_id UUID,
  old_values JSONB,
  new_values JSONB,
  ip_address VARCHAR(45),
  user_agent VARCHAR(500),
  request_id VARCHAR(64),
  trace_id VARCHAR(64),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (actor_type IN ('user', 'system', 'api_key', 'platform_admin'))
);

CREATE INDEX audit_log_org_occurred_idx ON audit_log (organization_id, occurred_at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_id) WHERE actor_id IS NOT NULL;
CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id);
CREATE INDEX audit_log_action_idx ON audit_log (action, occurred_at DESC);

-- Bloquear UPDATE/DELETE a nivel DB (HU-02.4 inmutabilidad)
-- Esto se enforce a nivel role en HU-25.6; aquí solo doc note.
