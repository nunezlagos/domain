-- migration: 000048_backfill_dedup_hash
-- description: Backfill content_hash para observations pre-000031 (HU-03.6)
-- breaking: false
-- estimated_duration: depende del volumen (secs-min)

CREATE EXTENSION IF NOT EXISTS pgcrypto;

UPDATE observations
SET content_hash = digest(
  COALESCE(content, '') || COALESCE(title, ''),
  'sha256'
)
WHERE content_hash IS NULL
  AND deleted_at IS NULL;
