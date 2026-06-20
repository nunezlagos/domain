-- migration: create_agent_run_prompts
-- author: mnunez@saargo.com
-- issue: REQ-42.4 captura del prompt efectivo que cae al agente por run/iteracion
-- description: persistir el prompt REAL que la plataforma ensambla y envia al
--   LLM en cada iteracion de un agent_run (system_prompt resuelto + mensajes
--   serializados + tools). Distinto de `captured_prompts` (raw_text del USUARIO
--   en su IDE, mig 000104). Aca se guarda lo que el ORQUESTADOR manda al MODELO,
--   para auditar/reproducir runs y, cruzando con captured_prompts, ensenarle al
--   usuario orquestador como su prompt crudo se transformo en el prompt final.
--   Una fila por (agent_run_id, iteration). Single-org: sin organization_id ni
--   RLS (mig 000132 deshabilito RLS, mig 000142 dropeo organization_id global).
--   FK a agent_runs(id) ON DELETE CASCADE siguiendo el patron de agent_run_logs.
--   Trigger set_updated_at() ya existe globalmente (mig 000001-000003).
-- breaking: false (tabla nueva; no toca tablas existentes)
-- estimated_duration: <1s

BEGIN;

CREATE TABLE IF NOT EXISTS agent_run_prompts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  -- iteracion del loop de completion (0 = pre-flight, 1..N = cada llamada al LLM)
  iteration INT NOT NULL DEFAULT 0,
  -- modelo destino de esta llamada (puede variar si el agente cambia de modelo)
  model VARCHAR(100),
  -- system prompt resuelto del agente en el momento de la llamada
  system_prompt TEXT NOT NULL DEFAULT '',
  -- mensajes ensamblados (user/assistant/tool) serializados tal cual van al LLM
  messages JSONB NOT NULL DEFAULT '[]',
  -- slugs de las skills/tools expuestas como tool defs en esta llamada
  tool_slugs TEXT[] NOT NULL DEFAULT '{}',
  -- proxy de tamano hasta tener contabilidad real de tokens del request
  char_count INT NOT NULL DEFAULT 0,
  estimated_tokens INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT agent_run_prompts_iteration_check CHECK (iteration >= 0),
  -- una fila por iteracion dentro de un run (idempotencia del snapshot)
  CONSTRAINT agent_run_prompts_run_iter_key UNIQUE (agent_run_id, iteration)
);

CREATE TRIGGER set_updated_at_agent_run_prompts
  BEFORE UPDATE ON agent_run_prompts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- acceso por run (caso principal: ver todos los prompts de un run en orden)
CREATE INDEX IF NOT EXISTS agent_run_prompts_run_idx
  ON agent_run_prompts (agent_run_id, iteration);
-- analitica temporal (ultimos prompts ensamblados)
CREATE INDEX IF NOT EXISTS agent_run_prompts_created_idx
  ON agent_run_prompts (created_at DESC);

COMMIT;
