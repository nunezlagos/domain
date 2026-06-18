-- Revertir: restaurar NOT NULL en organization_id y la UNIQUE(organization_id, alert_date).
-- Nota: falla si existen filas con organization_id NULL (alertas enviadas tras el up).
-- En una DB fresca (roundtrip) no hay NULLs, así que es seguro.
ALTER TABLE cost_alerts_sent
  DROP CONSTRAINT IF EXISTS cost_alerts_sent_alert_date_key;

ALTER TABLE cost_alerts_sent
  ADD CONSTRAINT cost_alerts_sent_organization_id_alert_date_key
  UNIQUE (organization_id, alert_date);

ALTER TABLE cost_alerts_sent
  ALTER COLUMN organization_id SET NOT NULL;
