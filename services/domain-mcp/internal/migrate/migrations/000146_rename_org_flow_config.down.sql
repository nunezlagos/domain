






BEGIN;

ALTER TABLE flow_config RENAME TO org_flow_config;
ALTER SEQUENCE flow_config_id_seq RENAME TO org_flow_config_id_seq;
ALTER INDEX flow_config_pkey RENAME TO org_flow_config_pkey;
ALTER INDEX flow_config_status_idx RENAME TO org_flow_config_status_idx;
ALTER TABLE org_flow_config
  RENAME CONSTRAINT flow_config_max_flow_duration_seconds_check
  TO org_flow_config_max_flow_duration_seconds_check;

COMMIT;
