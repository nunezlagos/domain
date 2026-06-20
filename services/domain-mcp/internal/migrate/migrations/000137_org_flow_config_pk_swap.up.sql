-- migration: org_flow_config_pk_swap
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase B — per-consumer cleanup)
-- description: org_flow_config tiene PK = organization_id, idéntico problema
--   al de org_cost_alert_thresholds. Intercambiamos a surrogate id BIGSERIAL
--   para permitir DROP COLUMN organization_id en Fase C.
--   Consumidores (usage/service, flow/budget, runner/flow/recovery) ya
--   consultan con LIMIT 1 sin organization_id (issue-21.5 single-org).
--   Single-org: como mucho 1 row activa (max_flow_duration_seconds global).
-- breaking: false (código no toca la PK)
-- estimated_duration: <1s

ALTER TABLE org_flow_config
  DROP CONSTRAINT org_flow_config_pkey;

ALTER TABLE org_flow_config
  ADD COLUMN id BIGSERIAL;

SELECT setval(
  pg_get_serial_sequence('org_flow_config', 'id'),
  COALESCE((SELECT MAX(id) FROM org_flow_config), 0) + 1,
  false
);

ALTER TABLE org_flow_config
  ALTER COLUMN id SET NOT NULL;

ALTER TABLE org_flow_config
  ADD CONSTRAINT org_flow_config_pkey PRIMARY KEY (id);

ALTER TABLE org_flow_config
  ADD CONSTRAINT org_flow_config_org_unique UNIQUE (organization_id);

ALTER TABLE org_flow_config
  ALTER COLUMN organization_id DROP NOT NULL;
