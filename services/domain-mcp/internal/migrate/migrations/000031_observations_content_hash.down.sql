DROP INDEX IF EXISTS observations_dedup_hash_uniq;
ALTER TABLE observations DROP COLUMN IF EXISTS content_hash;
