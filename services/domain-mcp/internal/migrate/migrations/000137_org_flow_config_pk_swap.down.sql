


ALTER TABLE org_flow_config
  DROP CONSTRAINT IF EXISTS org_flow_config_org_unique;

ALTER TABLE org_flow_config
  DROP CONSTRAINT org_flow_config_pkey;

ALTER TABLE org_flow_config
  DROP COLUMN id;

ALTER TABLE org_flow_config
  ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE org_flow_config
  ADD CONSTRAINT org_flow_config_pkey PRIMARY KEY (organization_id);
