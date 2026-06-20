-- Revertir: re-crear las columnas como nullable UUID. NO restaura los valores
-- (esos se perdieron en el up). En roundtrip (DB fresca) no hay filas, así que
-- el reverse es seguro. En DB con datos, queda una columna vacía.
ALTER TABLE cost_alerts_sent           ADD COLUMN IF NOT EXISTS organization_id UUID;
ALTER TABLE org_cost_alert_thresholds  ADD COLUMN IF NOT EXISTS organization_id UUID;
ALTER TABLE org_flow_config            ADD COLUMN IF NOT EXISTS organization_id UUID;
ALTER TABLE usage_counters             ADD COLUMN IF NOT EXISTS organization_id UUID;
ALTER TABLE org_enrollment_tokens      ADD COLUMN IF NOT EXISTS organization_id UUID;
