-- migration: create_cron_executions (down)
-- author: nunezlagos
-- issue: issue-10.1
-- description: elimina cron_executions (warning: pierde historial de ejecuciones)
-- breaking: false
-- estimated_duration: <1s

DROP TABLE IF EXISTS cron_executions CASCADE;
