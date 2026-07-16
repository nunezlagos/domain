-- migration: 000271_prompts_user_modified (down)
-- author: nunezlagos
-- issue: DOMAINSERV-27
-- description: revierte la columna is_user_modified de prompts.
-- breaking: no
-- duration: <1s
ALTER TABLE prompts
  DROP COLUMN IF EXISTS is_user_modified;
