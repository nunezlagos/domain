-- migration: 000150_create_agent_run_prompts
-- author: NunezLagos
-- issue: legacy
-- description: crea la tabla agent_run_prompts para persistir prompts por corrida de agente
-- breaking: no
-- estimated_duration: unknown

BEGIN;

CREATE TABLE IF NOT EXISTS agent_run_prompts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,

  iteration INT NOT NULL DEFAULT 0,

  model VARCHAR(100),

  system_prompt TEXT NOT NULL DEFAULT '',

  messages JSONB NOT NULL DEFAULT '[]',

  tool_slugs TEXT[] NOT NULL DEFAULT '{}',

  char_count INT NOT NULL DEFAULT 0,
  estimated_tokens INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT agent_run_prompts_iteration_check CHECK (iteration >= 0),

  CONSTRAINT agent_run_prompts_run_iter_key UNIQUE (agent_run_id, iteration)
);

CREATE TRIGGER set_updated_at_agent_run_prompts
  BEFORE UPDATE ON agent_run_prompts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


CREATE INDEX IF NOT EXISTS agent_run_prompts_run_idx
  ON agent_run_prompts (agent_run_id, iteration);

CREATE INDEX IF NOT EXISTS agent_run_prompts_created_idx
  ON agent_run_prompts (created_at DESC);

COMMIT;
