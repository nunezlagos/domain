-- migration: create_intake_payloads
-- author: nunezlagos
-- issue: HU-04.8
-- description: pipeline de ingesta unificada para requerimientos heterogéneos
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE intake_payloads (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source VARCHAR(30) NOT NULL
    CHECK (source IN ('agent','email','webhook','slack','sheet','manual')),
  source_ref TEXT,                       -- message-id / webhook-id / sheet-row-id
  organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
  submitted_by VARCHAR(255),             -- email o user_id externo
  raw_text TEXT NOT NULL,
  raw_payload JSONB NOT NULL DEFAULT '{}',
  status VARCHAR(30) NOT NULL DEFAULT 'received'
    CHECK (status IN ('received','classifying','classified','deduping','structuring',
                      'pending_review','approved','rejected','committed','failed')),
  classified_type VARCHAR(20),           -- feat|fix|hotfix|chore|refactor|docs
  classified_severity VARCHAR(20),       -- low|medium|high|critical
  classified_confidence NUMERIC(3,2),
  classification_reasoning TEXT,
  needs_clarification BOOLEAN NOT NULL DEFAULT false,
  proposed_title TEXT,
  proposed_description TEXT,
  proposed_req_slug VARCHAR(50),
  proposed_hu_draft JSONB,
  dedup_candidates JSONB NOT NULL DEFAULT '[]',
  merge_action VARCHAR(30),              -- create_new|append_to_hu|merge_with_req
  reviewer_id UUID REFERENCES users(id) ON DELETE SET NULL,
  reviewed_at TIMESTAMPTZ,
  rejection_reason TEXT,
  committed_req_id UUID REFERENCES requirements(id) ON DELETE SET NULL,
  committed_hu_id UUID REFERENCES user_stories(id) ON DELETE SET NULL,
  failure_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX intake_payloads_status_idx ON intake_payloads (status, created_at);
CREATE INDEX intake_payloads_source_idx ON intake_payloads (source, created_at);
CREATE INDEX intake_payloads_reviewer_idx ON intake_payloads (reviewer_id)
  WHERE status = 'pending_review';

CREATE TRIGGER set_updated_at_intake_payloads
  BEFORE UPDATE ON intake_payloads
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE intake_attachments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  intake_id UUID NOT NULL REFERENCES intake_payloads(id) ON DELETE CASCADE,
  filename VARCHAR(255) NOT NULL,
  mime_type VARCHAR(127) NOT NULL,
  size_bytes BIGINT NOT NULL,
  s3_key TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX intake_attachments_intake_idx ON intake_attachments (intake_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON intake_payloads TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON intake_attachments TO app_user;
