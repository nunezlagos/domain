DROP INDEX IF EXISTS idx_audit_log_org_action;
DROP INDEX IF EXISTS idx_audit_log_org_time;
ALTER TABLE audit_log DROP COLUMN IF EXISTS origin_org_id;
