-- migration: system_state_and_run_metadata (DOWN)
-- WARNING: drop de metadata pierde flags standalone+reason.

BEGIN;

ALTER TABLE agent_runs DROP COLUMN IF EXISTS metadata;
DROP TABLE IF EXISTS system_state;

COMMIT;
