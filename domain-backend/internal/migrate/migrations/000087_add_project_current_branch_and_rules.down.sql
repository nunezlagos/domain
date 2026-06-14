DROP INDEX IF EXISTS projects_rules_gin_idx;
ALTER TABLE projects
  DROP COLUMN IF EXISTS rules,
  DROP COLUMN IF EXISTS current_branch;
