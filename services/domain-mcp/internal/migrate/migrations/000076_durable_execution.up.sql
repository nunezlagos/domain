-- migration: durable_execution
-- author: nunezlagos
-- issue: issue-09.6
-- description: output_compressed + output_s3_key for flow_run_steps
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE flow_run_steps ADD COLUMN IF NOT EXISTS output_compressed BYTEA;
ALTER TABLE flow_run_steps ADD COLUMN IF NOT EXISTS output_s3_key VARCHAR(500);
