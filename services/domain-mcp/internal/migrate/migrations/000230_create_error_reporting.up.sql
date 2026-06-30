CREATE TABLE IF NOT EXISTS error_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source text NOT NULL,
  category text NOT NULL,
  severity text NOT NULL DEFAULT 'warn'
    CHECK (severity IN ('debug','info','warn','error','critical')),
  message text NOT NULL,
  stack_trace text NULL,
  fingerprint bytea NOT NULL,
  dedup_count int NOT NULL DEFAULT 1,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  workflow_id text NULL,
  project_id uuid NULL,
  recoverable boolean NOT NULL DEFAULT true,
  remediation text NULL,
  metadata jsonb NOT NULL DEFAULT '{}'
);

-- UNIQUE en fingerprint: requerido por el upsert ON CONFLICT (fingerprint)
-- que incrementa dedup_count en vez de insertar duplicados.
CREATE UNIQUE INDEX IF NOT EXISTS uq_error_events_fingerprint
  ON error_events (fingerprint);
CREATE INDEX IF NOT EXISTS idx_error_events_category_last
  ON error_events (category, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_error_events_severity_last
  ON error_events (severity, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_error_events_workflow
  ON error_events (workflow_id)
  WHERE workflow_id IS NOT NULL;
