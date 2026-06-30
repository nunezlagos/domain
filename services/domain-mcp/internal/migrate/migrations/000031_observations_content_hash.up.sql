






ALTER TABLE observations
  ADD COLUMN IF NOT EXISTS content_hash BYTEA;






CREATE UNIQUE INDEX IF NOT EXISTS observations_dedup_hash_uniq
  ON observations (project_id, observation_type, content_hash)
  WHERE content_hash IS NOT NULL AND deleted_at IS NULL;
