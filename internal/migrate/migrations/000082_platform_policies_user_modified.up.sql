-- migration: platform_policies_user_modified
-- author: nunezlagos
-- issue: issue-01.8
-- description: agrega is_user_modified a platform_policies para que el seeder no pise ediciones manuales
-- breaking: false
-- estimated_duration: 1s (tabla pequeña <100 rows)

ALTER TABLE platform_policies
  ADD COLUMN IF NOT EXISTS is_user_modified BOOLEAN NOT NULL DEFAULT FALSE;
