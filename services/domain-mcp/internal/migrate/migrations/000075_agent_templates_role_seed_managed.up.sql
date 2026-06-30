






BEGIN;

ALTER TABLE agent_templates
  ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'phase-worker'
    CHECK (role IN ('orchestrator', 'phase-worker'));

ALTER TABLE agent_templates
  ADD COLUMN IF NOT EXISTS seed_managed BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE agent_templates
  ADD COLUMN IF NOT EXISTS is_user_modified BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE agent_templates
  ADD COLUMN IF NOT EXISTS seed_version INT;





CREATE UNIQUE INDEX IF NOT EXISTS agent_templates_orchestrator_unique_idx
  ON agent_templates (organization_id) WHERE role = 'orchestrator';

COMMIT;
