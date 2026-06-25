



ALTER TABLE org_cost_alert_thresholds
  DROP CONSTRAINT IF EXISTS org_cost_alert_thresholds_org_unique;

ALTER TABLE org_cost_alert_thresholds
  DROP CONSTRAINT org_cost_alert_thresholds_pkey;

ALTER TABLE org_cost_alert_thresholds
  DROP COLUMN id;

ALTER TABLE org_cost_alert_thresholds
  ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE org_cost_alert_thresholds
  ADD CONSTRAINT org_cost_alert_thresholds_pkey PRIMARY KEY (organization_id);
