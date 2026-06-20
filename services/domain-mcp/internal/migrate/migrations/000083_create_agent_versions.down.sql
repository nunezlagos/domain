-- migration: create_agent_versions (down)
-- author: nunezlagos
-- issue: issue-08.1
-- description: elimina agent_versions (warning: pierde historial de versiones)
-- breaking: false
-- estimated_duration: <1s

DROP TABLE IF EXISTS agent_versions CASCADE;
