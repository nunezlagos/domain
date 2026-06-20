-- migration: create_hu_drafts
-- author: nunezlagos
-- issue: HU-04.7
-- description: state machine persistente del wizard interactivo de HU
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE hu_drafts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  mode VARCHAR(20) NOT NULL
    CHECK (mode IN ('feature','bug-fix','refactor','doc','rfc')),
  initial_idea TEXT NOT NULL,
  answers JSONB NOT NULL DEFAULT '{}',
  current_step INT NOT NULL DEFAULT 0,
  total_steps INT NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'in_progress'
    CHECK (status IN ('in_progress','finished','committed','expired','abandoned')),
  pending_clarifications JSONB NOT NULL DEFAULT '[]',
  preview JSONB,
  target_path TEXT,
  committed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX hu_drafts_created_by_status_idx ON hu_drafts (created_by, status);
CREATE INDEX hu_drafts_expires_at_idx ON hu_drafts (expires_at)
  WHERE status = 'in_progress';

CREATE TRIGGER set_updated_at_hu_drafts
  BEFORE UPDATE ON hu_drafts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE hu_draft_steps_log (
  id BIGSERIAL PRIMARY KEY,
  draft_id UUID NOT NULL REFERENCES hu_drafts(id) ON DELETE CASCADE,
  step_key VARCHAR(50) NOT NULL,
  question TEXT NOT NULL,
  options JSONB,
  answer JSONB,
  answered_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX hu_draft_steps_log_draft_idx ON hu_draft_steps_log (draft_id, id);

GRANT SELECT, INSERT, UPDATE, DELETE ON hu_drafts TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON hu_draft_steps_log TO app_user;
GRANT USAGE, SELECT ON SEQUENCE hu_draft_steps_log_id_seq TO app_user;
