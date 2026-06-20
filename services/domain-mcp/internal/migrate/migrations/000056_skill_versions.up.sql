-- migration: skill_versions_pinned
-- author: nunezlagos
-- issue: HU-05.3
-- description: agrega pinned_version a skills (skill_versions ya existe desde 000011)
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE skills ADD COLUMN IF NOT EXISTS pinned_version INT;
