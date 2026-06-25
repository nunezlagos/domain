









ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS last_known_head VARCHAR(40),  -- sha1 git
  ADD COLUMN IF NOT EXISTS last_seen_at    TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_seen_branch VARCHAR(120),
  ADD COLUMN IF NOT EXISTS last_seen_cwd    VARCHAR(500);

CREATE INDEX IF NOT EXISTS projects_last_seen_at_idx
  ON projects (organization_id, last_seen_at DESC)
  WHERE deleted_at IS NULL;
