-- migration: org_cost_alert_thresholds_pk_swap
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase B — per-consumer cleanup)
-- description: org_cost_alert_thresholds tiene PK = organization_id, lo cual
--   bloquea el DROP COLUMN de Fase C. Intercambiamos la PK por una surrogate
--   (id BIGSERIAL) y dejamos organization_id como UNIQUE nullable.
--   El código de usagealerts/threshold_checker ya NO usa organization_id
--   (EnableCostThreshold/GetCostThreshold usan LIMIT 1, sin WHERE org_id).
--   Single-org: como mucho 1 row, garantizada por la UNIQUE constraint.
-- breaking: false (código no toca la PK)
-- estimated_duration: <1s

ALTER TABLE org_cost_alert_thresholds
  DROP CONSTRAINT org_cost_alert_thresholds_pkey;

ALTER TABLE org_cost_alert_thresholds
  ADD COLUMN id BIGSERIAL;

-- Re-llenar la secuencia a partir del máximo id existente (defensa por si la
-- tabla tenía filas con PK anterior; en una DB recién migrada está vacío).
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
