-- migration: create_dead_letter_queue
-- author: nunezlagos
-- issue: issue-09.4
-- description: DLQ para steps con fallo permanente (retries agotados sin política de recuperación)
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS dead_letter_queue (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  flow_run_id UUID REFERENCES flow_runs(id) ON DELETE CASCADE,
  flow_slug VARCHAR(120) NOT NULL,
  step_key VARCHAR(120) NOT NULL,
  error TEXT NOT NULL,
  errors JSONB NOT NULL DEFAULT '[]',
  retry_count INT NOT NULL DEFAULT 0,
  failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- domain-lint-ignore-next: require-concurrent-index-creation
-- reason: tabla nueva sin tráfico
CREATE INDEX IF NOT EXISTS dead_letter_queue_org_pending_idx
  ON dead_letter_queue (organization_id, failed_at DESC)
  WHERE resolved_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON dead_letter_queue TO app_user;
