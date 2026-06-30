






CREATE TABLE agent_run_logs (
  id BIGSERIAL PRIMARY KEY,
  agent_run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  iteration INT NOT NULL,
  event_type VARCHAR(30) NOT NULL
    CHECK (event_type IN ('llm_call','tool_call','tool_result','error','final')),



  payload JSONB NOT NULL DEFAULT '{}',
  tokens_input INT NOT NULL DEFAULT 0,
  tokens_output INT NOT NULL DEFAULT 0,
  latency_ms INT NOT NULL DEFAULT 0,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX agent_run_logs_run_iter_idx ON agent_run_logs (agent_run_id, iteration);
CREATE INDEX agent_run_logs_occurred_idx ON agent_run_logs (occurred_at DESC);

GRANT SELECT, INSERT ON agent_run_logs TO app_user;
GRANT USAGE, SELECT ON SEQUENCE agent_run_logs_id_seq TO app_user;
GRANT ALL ON agent_run_logs TO app_admin;
GRANT ALL ON SEQUENCE agent_run_logs_id_seq TO app_admin;
