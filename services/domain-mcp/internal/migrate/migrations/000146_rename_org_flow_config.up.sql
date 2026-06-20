-- migration: rename_org_flow_config_to_flow_config
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase D — rename cleanup)
-- description: rename table `org_flow_config` → `flow_config` (cleanup
--   post-Fase C). El prefijo `org_` era vestigial del diseño multi-tenant;
--   en single-org es una config global. La tabla tiene 1 row activo
--   (singleton con LIMIT 1 en todas las queries). El rename preserva
--   todos los datos y referencias (PK, índices, constraints, trigger).
--
--   Cambios:
--   1. ALTER TABLE org_flow_config RENAME TO flow_config
--   2. Renombrar sequence (postgres usa nombre_<table>_id_seq convention)
--   3. Renombrar índices (Postgres los nombra con la tabla)
--   4. Renombrar constraint CHECK
--
--   down: RENAME reverso (atómico).
-- breaking: false (cambio de naming interno; API pública no afectada)
-- estimated_duration: <1s

BEGIN;

ALTER TABLE org_flow_config RENAME TO flow_config;
ALTER SEQUENCE org_flow_config_id_seq RENAME TO flow_config_id_seq;
ALTER INDEX org_flow_config_pkey RENAME TO flow_config_pkey;
ALTER INDEX org_flow_config_status_idx RENAME TO flow_config_status_idx;
ALTER TABLE flow_config
  RENAME CONSTRAINT org_flow_config_max_flow_duration_seconds_check
  TO flow_config_max_flow_duration_seconds_check;

COMMIT;
