DROP INDEX IF EXISTS idx_http_request_log_workflow;
ALTER TABLE http_request_log DROP COLUMN IF EXISTS workflow_id;