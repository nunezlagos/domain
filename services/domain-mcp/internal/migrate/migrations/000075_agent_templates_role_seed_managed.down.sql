

BEGIN;

DROP INDEX IF EXISTS agent_templates_orchestrator_unique_idx;
ALTER TABLE agent_templates DROP COLUMN IF EXISTS seed_version;
ALTER TABLE agent_templates DROP COLUMN IF EXISTS is_user_modified;
ALTER TABLE agent_templates DROP COLUMN IF EXISTS seed_managed;
ALTER TABLE agent_templates DROP COLUMN IF EXISTS role;

COMMIT;
