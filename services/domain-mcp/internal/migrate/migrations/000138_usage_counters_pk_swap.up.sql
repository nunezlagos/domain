












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
