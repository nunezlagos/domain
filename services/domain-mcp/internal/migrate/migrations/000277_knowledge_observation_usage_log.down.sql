-- migration: 000277_knowledge_observation_usage_log (down)
-- author: nunezlagos
-- issue: DOMAINSERV-145
-- description: revierte la tabla de señal de consumo. Reversible sin pérdida
--   de datos de usuario: la tabla es telemetría derivada, no fuente de verdad
--   de nada. Los índices y el UNIQUE caen con ella.
-- breaking: no
-- estimated_duration: <1s

DROP TABLE IF EXISTS knowledge_observation_usage_log CASCADE;
