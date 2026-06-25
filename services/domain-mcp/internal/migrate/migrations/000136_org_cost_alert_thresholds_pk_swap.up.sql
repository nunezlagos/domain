











ALTER TABLE org_cost_alert_thresholds
  DROP CONSTRAINT org_cost_alert_thresholds_pkey;

ALTER TABLE org_cost_alert_thresholds
  ADD COLUMN id BIGSERIAL;



SELECT setval(
  pg_get_serial_sequence('org_cost_alert_thresholds', 'id'),
  COALESCE((SELECT MAX(id) FROM org_cost_alert_thresholds), 0) + 1,
  false
);

ALTER TABLE org_cost_alert_thresholds
  ALTER COLUMN id SET NOT NULL;

ALTER TABLE org_cost_alert_thresholds
  ADD CONSTRAINT org_cost_alert_thresholds_pkey PRIMARY KEY (id);

ALTER TABLE org_cost_alert_thresholds
  ADD CONSTRAINT org_cost_alert_thresholds_org_unique UNIQUE (organization_id);

ALTER TABLE org_cost_alert_thresholds
  ALTER COLUMN organization_id DROP NOT NULL;
