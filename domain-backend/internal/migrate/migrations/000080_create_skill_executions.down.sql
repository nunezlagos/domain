-- migration: create_skill_executions (down)
-- author: nunezlagos
-- issue: issue-05.5
-- description: revierte skill_executions (warning: pierde historial de ejecuciones)
-- breaking: false
-- estimated_duration: <1s

DROP TABLE IF EXISTS skill_executions CASCADE;
