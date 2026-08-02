-- migration: 000279_agent_runs_client_reported (down)
-- author: nunezlagos
-- issue: DOMAINSERV-147
-- description: quita las columnas del reporte de runs del cliente. Los runs
--   server-side de 000015 quedan intactos: nunca usaron estas columnas más allá
--   del DEFAULT 'server'.
--
--   Lo que SÍ se pierde al bajar es la atribución de los runs reportados por el
--   cliente desde el deploy: la fila sobrevive con sus tokens y su costo, pero
--   sin project, sin modelo y sin la marca de quién la ejecutó. O sea, al bajar
--   quedan indistinguibles de un run del server. Es asumido y declarado: el
--   rollback no puede reconstruir una dimensión que solo existía en estas
--   columnas.
-- breaking: no
-- estimated_duration: <1s

DROP INDEX IF EXISTS agent_runs_project_source_idx;

ALTER TABLE agent_runs
  DROP COLUMN IF EXISTS model,
  DROP COLUMN IF EXISTS project_id,
  DROP COLUMN IF EXISTS source;
