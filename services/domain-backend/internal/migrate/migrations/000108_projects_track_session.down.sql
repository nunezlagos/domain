DROP INDEX IF EXISTS projects_last_seen_at_idx;
ALTER TABLE projects
  DROP COLUMN IF EXISTS last_seen_cwd,
  DROP COLUMN IF EXISTS last_seen_branch,
  DROP COLUMN IF EXISTS last_seen_at,
  DROP COLUMN IF EXISTS last_known_head;
