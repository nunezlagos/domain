DROP TRIGGER IF EXISTS set_updated_at_usage_counters ON usage_counters;
DROP TABLE IF EXISTS usage_counters;

ALTER TABLE organizations
  DROP COLUMN IF EXISTS plan_started_at,
  DROP COLUMN IF EXISTS custom_limits,
  DROP COLUMN IF EXISTS plan_id;

DROP TRIGGER IF EXISTS set_updated_at_plans ON plans;
DROP TABLE IF EXISTS plans;
