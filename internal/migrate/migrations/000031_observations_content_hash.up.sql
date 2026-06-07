-- migration: observations_content_hash
-- author: mnunez@saargo.com
-- issue: HU-03.6
-- description: content_hash SHA-256 normalizado para dedup defense-in-depth
-- breaking: false
-- estimated_duration: <1s (empty table)

ALTER TABLE observations
  ADD COLUMN IF NOT EXISTS content_hash BYTEA;

-- Unique partial: solo aplica a observations vivas (deleted_at IS NULL) y con
-- hash seteado. Permite que content idéntico aparezca en distintos projects.
CREATE UNIQUE INDEX IF NOT EXISTS observations_dedup_hash_uniq
  ON observations (project_id, observation_type, content_hash)
  WHERE content_hash IS NOT NULL AND deleted_at IS NULL;
