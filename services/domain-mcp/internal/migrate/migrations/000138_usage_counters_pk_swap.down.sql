-- Revertir: restaurar PK compuesta (organization_id, period_start).
-- Nota: falla si hay más de 1 fila para la misma (org, period).

ALTER TABLE usage_counters
  DROP CONSTRAINT IF EXISTS usage_counters_period_unique;

ALTER TABLE usage_counters
  DROP CONSTRAINT usage_counters_pkey;

ALTER TABLE usage_counters
  DROP COLUMN id;

ALTER TABLE usage_counters
  ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE usage_counters
  ADD CONSTRAINT usage_counters_pkey PRIMARY KEY (organization_id, period_start);
