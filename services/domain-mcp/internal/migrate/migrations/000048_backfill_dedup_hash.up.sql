




CREATE EXTENSION IF NOT EXISTS pgcrypto;

UPDATE observations
SET content_hash = digest(COALESCE(content, ''), 'sha256')
WHERE content_hash IS NULL
  AND deleted_at IS NULL;
