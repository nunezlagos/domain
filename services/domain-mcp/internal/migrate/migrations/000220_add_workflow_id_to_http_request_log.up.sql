ALTER TABLE http_request_log
  ADD COLUMN IF NOT EXISTS workflow_id text NULL;

CREATE INDEX IF NOT EXISTS idx_http_request_log_workflow
  ON http_request_log (workflow_id)
  WHERE workflow_id IS NOT NULL;