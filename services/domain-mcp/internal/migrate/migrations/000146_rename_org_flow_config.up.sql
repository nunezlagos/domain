


















BEGIN;

ALTER TABLE org_flow_config RENAME TO flow_config;
ALTER SEQUENCE org_flow_config_id_seq RENAME TO flow_config_id_seq;
ALTER INDEX org_flow_config_pkey RENAME TO flow_config_pkey;
ALTER INDEX org_flow_config_status_idx RENAME TO flow_config_status_idx;
ALTER TABLE flow_config
  RENAME CONSTRAINT org_flow_config_max_flow_duration_seconds_check
  TO flow_config_max_flow_duration_seconds_check;

COMMIT;
