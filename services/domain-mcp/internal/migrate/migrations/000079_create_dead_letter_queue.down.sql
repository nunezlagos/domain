-- migration: create_dead_letter_queue (down)
-- author: nunezlagos
-- issue: issue-09.4
-- description: revierte DLQ (warning: pierde registros pendientes)
-- breaking: false
-- estimated_duration: <1s

DROP TABLE IF EXISTS dead_letter_queue CASCADE;
