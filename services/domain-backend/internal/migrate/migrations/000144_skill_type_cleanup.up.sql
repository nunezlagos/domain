-- migration: skill_type_cleanup
-- author: mnunez@saargo.com
-- issue: REQ-35 architectural-debt (issue-35.2 skill-model-decision-record)
-- description: Día 1 del plan de migración gradual de Opción A del
--   RFC 0008 (skill model simplification). Convierte skills existentes con
--   tipos deprecated (api / code / mcp_tool) a 'prompt'. NO dropea la
--   columna ni cambia el CHECK constraint (eso es Día 7 cuando el code
--   rechaza los 3 tipos).
--
--   El RFC argumenta que el skill_runner server-side está NUNCA USADO
--   (0 ejecuciones vs 245 agent_runs + 1023 flow_runs) y el 94% de los
--   skills son TypePrompt. Los 3 stubs prometen funcionalidad que no
--   entregan, confundir al user e inflan código.
--
--   Migration IDEMPOTENTE: si se corre 2 veces, la segunda vez afecta 0
--   filas (ya están todas en 'prompt'). Reversible: el down restaura
--   los valores originales leyendo backup_temp.
--
--   No afecta soft-deleted skills (deleted_at IS NOT NULL) — el usuario
--   podría querer revisarlos antes de migrar.
-- breaking: false (no cambia schema, solo valores)
-- estimated_duration: <1s

BEGIN;

-- Backup de los valores originales en una tabla temporal para reversibilidad.
CREATE TEMP TABLE skill_type_backup AS
  SELECT id, skill_type AS old_type
  FROM skills
  WHERE skill_type IN ('api', 'code', 'mcp_tool')
    AND deleted_at IS NULL;

-- Convertir los stubs a 'prompt'. No tocamos deleted_at ni timestamps
-- para preservar auditoría.
UPDATE skills
SET skill_type = 'prompt',
    updated_at = NOW()
WHERE skill_type IN ('api', 'code', 'mcp_tool')
  AND deleted_at IS NULL;

-- Log cuántos rows se convirtieron (útil para auditoría post-deploy).
DO $$
DECLARE
    n INT;
BEGIN
    SELECT COUNT(*) INTO n FROM skill_type_backup;
    RAISE NOTICE 'skill_type_cleanup: % filas convertidas de (api/code/mcp_tool) a prompt', n;
END $$;

COMMIT;
