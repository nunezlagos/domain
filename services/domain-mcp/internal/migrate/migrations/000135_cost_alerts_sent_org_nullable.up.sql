











ALTER TABLE cost_alerts_sent
  ALTER COLUMN organization_id DROP NOT NULL;

ALTER TABLE cost_alerts_sent
  DROP CONSTRAINT IF EXISTS cost_alerts_sent_organization_id_alert_date_key;

ALTER TABLE cost_alerts_sent
  ADD CONSTRAINT cost_alerts_sent_alert_date_key UNIQUE (alert_date);
