-- migration: 000289_flow_agent_scopes (down)
-- author: nunezlagos
-- issue: DOMAINSERV-218
-- description: revierte la tabla de scopes de edición por agente
-- breaking: no
-- estimated_duration: <1s
--
-- current_project_id() NO se dropea acá: la crearon la 000287 y la 000288 y sus policies la
-- siguen usando. Dropearla dejaría el RLS de knowledge y de webhooks apuntando a una función
-- inexistente.

DROP TABLE IF EXISTS flow_agent_scopes CASCADE;
