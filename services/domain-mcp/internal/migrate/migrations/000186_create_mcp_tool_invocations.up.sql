CREATE TABLE IF NOT EXISTS mcp_tool_invocations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tool_name text NOT NULL,
  principal_id uuid NULL,
  org_id uuid NULL,
  project_id uuid NULL,
  status text NOT NULL CHECK (status IN ('ok','error','circuit_open','rate_limited','cache_hit','cache_miss')),
  duration_ms int NOT NULL,
  error_code text NULL,
  error_message text NULL,
  args_hash bytea NULL,
  workflow_id text NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mcp_tool_invocations_tool_created
  ON mcp_tool_invocations (tool_name, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_mcp_tool_invocations_principal_created
  ON mcp_tool_invocations (principal_id, created_at DESC)
  WHERE principal_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_mcp_tool_invocations_status_non_ok
  ON mcp_tool_invocations (status)
  WHERE status <> 'ok';
