











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
