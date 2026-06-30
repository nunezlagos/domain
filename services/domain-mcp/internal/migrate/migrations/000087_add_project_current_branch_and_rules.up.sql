









ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS current_branch VARCHAR(120),
  ADD COLUMN IF NOT EXISTS rules TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS projects_rules_gin_idx
  ON projects USING GIN (rules) WHERE deleted_at IS NULL;
