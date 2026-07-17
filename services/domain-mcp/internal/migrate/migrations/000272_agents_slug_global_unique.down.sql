-- migration: 000272_agents_slug_global_unique (down)
-- author: nunezlagos
-- issue: DOMAINSERV-50
-- description: revierte el índice único global de agents.slug.
-- breaking: no
-- duration: <1s
DROP INDEX IF EXISTS agents_slug_global_uniq;
