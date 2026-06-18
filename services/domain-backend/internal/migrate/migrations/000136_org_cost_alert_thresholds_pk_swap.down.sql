-- Revertir: restaurar PK original sobre organization_id y dropear surrogate id.
-- Nota: falla si hay más de 1 fila (la PK original no lo permite).
-- En roundtrip (DB fresca) hay 0 filas, así que es seguro.

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
