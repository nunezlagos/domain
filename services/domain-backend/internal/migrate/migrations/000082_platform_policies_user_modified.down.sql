-- migration: platform_policies_user_modified (down)
-- author: nunezlagos
-- issue: issue-01.8
-- description: revierte is_user_modified de platform_policies
-- breaking: false
-- estimated_duration: 1s

ALTER TABLE platform_policies
  DROP COLUMN IF EXISTS is_user_modified;
