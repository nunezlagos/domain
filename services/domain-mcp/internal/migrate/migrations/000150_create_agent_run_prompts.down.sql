-- migration: create_agent_run_prompts (down)
-- author: mnunez@saargo.com
-- issue: REQ-42.4 captura del prompt efectivo que cae al agente por run/iteracion
-- description: revertir la creacion de agent_run_prompts. DROP CASCADE elimina
--   tambien el trigger set_updated_at_agent_run_prompts y los indices asociados.
--   Mismo patron que 000015_create_agent_runs.down.sql.
-- breaking: false
-- estimated_duration: <1s

DROP TABLE IF EXISTS agent_run_prompts CASCADE;
