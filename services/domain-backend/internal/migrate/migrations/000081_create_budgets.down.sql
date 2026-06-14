-- migration: create_budgets (down)
-- author: nunezlagos
-- issue: issue-15.2
-- description: revierte budgets (warning: pierde budgets configurados)
-- breaking: false
-- estimated_duration: <1s

DROP TABLE IF EXISTS budgets CASCADE;
