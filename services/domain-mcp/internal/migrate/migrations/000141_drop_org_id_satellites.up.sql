


















ALTER TABLE cost_alerts_sent           DROP COLUMN IF EXISTS organization_id;
ALTER TABLE org_cost_alert_thresholds  DROP COLUMN IF EXISTS organization_id;
ALTER TABLE org_flow_config            DROP COLUMN IF EXISTS organization_id;
ALTER TABLE usage_counters             DROP COLUMN IF EXISTS organization_id;
ALTER TABLE org_enrollment_tokens      DROP COLUMN IF EXISTS organization_id;



