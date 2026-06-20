-- migration: add_project_current_branch_and_rules
-- author: nunezlagos
-- issue: F4 (completar Project entity)
-- description: agrega current_branch + rules[] a projects.
--              current_branch: rama git actual del worktree local del dev.
--              rules: slugs de platform_policies que aplican a este project
--              (subset del set global; permite override por org/proyecto).
-- breaking: false (additive; defaults seguros)
-- estimated_duration: <1s

ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS current_branch VARCHAR(120),
  ADD COLUMN IF NOT EXISTS rules TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS projects_rules_gin_idx
  ON projects USING GIN (rules) WHERE deleted_at IS NULL;
