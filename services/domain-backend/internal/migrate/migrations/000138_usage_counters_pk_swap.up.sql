-- migration: usage_counters_pk_swap
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase B — per-consumer cleanup)
-- description: usage_counters tiene PK compuesta (organization_id, period_start)
--   que bloquea DROP COLUMN organization_id en Fase C. Intercambiamos a
--   surrogate id BIGSERIAL y dejamos UNIQUE(period_start) para garantizar
--   1 row por mes.
--   Consumidores (billing/service) ya usan el patrón single-org global keyed
--   por period_start (ver service.go:incrementCounter con ON CONFLICT(period_start)
--   y GetUsage WHERE period_start = $1).
-- breaking: false (código ya keyed por period_start)
-- estimated_duration: <1s

ALTER TABLE usage_counters
  DROP CONSTRAINT usage_counters_pkey;

ALTER TABLE usage_counters
  ADD COLUMN id BIGSERIAL;

SELECT setval(
  pg_get_serial_sequence('usage_counters', 'id'),
  COALESCE((SELECT MAX(id) FROM usage_counters), 0) + 1,
  false
);

ALTER TABLE usage_counters
  ALTER COLUMN id SET NOT NULL;

ALTER TABLE usage_counters
  ADD CONSTRAINT usage_counters_pkey PRIMARY KEY (id);

ALTER TABLE usage_counters
  ADD CONSTRAINT usage_counters_period_unique UNIQUE (period_start);

ALTER TABLE usage_counters
  ALTER COLUMN organization_id DROP NOT NULL;
