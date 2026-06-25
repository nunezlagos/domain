


ALTER TABLE cost_alerts_sent           ADD COLUMN IF NOT EXISTS organization_id UUID;
ALTER TABLE org_cost_alert_thresholds  ADD COLUMN IF NOT EXISTS organization_id UUID;
ALTER TABLE org_flow_config            ADD COLUMN IF NOT EXISTS organization_id UUID;
ALTER TABLE usage_counters             ADD COLUMN IF NOT EXISTS organization_id UUID;
ALTER TABLE org_enrollment_tokens      ADD COLUMN IF NOT EXISTS organization_id UUID;
