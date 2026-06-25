






















BEGIN;


CREATE TEMP TABLE skill_type_backup AS
  SELECT id, skill_type AS old_type
  FROM skills
  WHERE skill_type IN ('api', 'code', 'mcp_tool')
    AND deleted_at IS NULL;



UPDATE skills
SET skill_type = 'prompt',
    updated_at = NOW()
WHERE skill_type IN ('api', 'code', 'mcp_tool')
  AND deleted_at IS NULL;


DO $$
DECLARE
    n INT;
BEGIN
    SELECT COUNT(*) INTO n FROM skill_type_backup;
    RAISE NOTICE 'skill_type_cleanup: % filas convertidas de (api/code/mcp_tool) a prompt', n;
END $$;

COMMIT;
