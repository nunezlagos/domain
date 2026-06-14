-- migration: agent_templates_role_seed_managed
-- author: nunezlagos
-- issue: issue-08.10 sdd-pipeline-orchestrator
-- description: agrega role (orchestrator|phase-worker) + seed_managed + is_user_modified + seed_version a agent_templates. UNIQUE INDEX parcial garantiza único orchestrator por org. Necesario para sdd-orchestrator + 9 sdd-* phase-workers (RFC 0006).
-- breaking: false
-- estimated_duration: <1s (aditivo, sin lock largo)

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

-- UNIQUE INDEX parcial: máximo 1 row con role='orchestrator' por org.
-- Asegura que sdd-orchestrator sea único entry point (RFC 0006 ADR-1).
-- squawk-ignore: require-concurrent-index-creation
-- reason: tabla tiene 10 rows tipicamente per-org, lock breve aceptable
CREATE UNIQUE INDEX IF NOT EXISTS agent_templates_orchestrator_unique_idx
  ON agent_templates (organization_id) WHERE role = 'orchestrator';

COMMIT;
