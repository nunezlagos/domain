






ALTER TABLE flow_run_steps ADD COLUMN IF NOT EXISTS output_compressed BYTEA;
ALTER TABLE flow_run_steps ADD COLUMN IF NOT EXISTS output_s3_key VARCHAR(500);
