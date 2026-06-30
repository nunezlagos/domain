CREATE TABLE IF NOT EXISTS http_request_log (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id uuid UNIQUE,
  method text NOT NULL,
  path text NOT NULL,
  status int NOT NULL,
  duration_ms int NOT NULL,
  principal_id uuid NULL,
  bytes_in int NOT NULL DEFAULT 0,
  bytes_out int NOT NULL DEFAULT 0,
  user_agent text NULL,
  captured_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_http_request_log_path_captured
  ON http_request_log (path, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_http_request_log_status_captured
  ON http_request_log (status, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_http_request_log_principal
  ON http_request_log (principal_id)
  WHERE principal_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS function_calls (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  fn_name text NOT NULL,
  pkg text NOT NULL,
  called_at timestamptz NOT NULL DEFAULT now(),
  duration_us int NOT NULL,
  status text NOT NULL CHECK (status IN ('ok','error','panic')),
  error_message text NULL,
  args_hash bytea NULL,
  workflow_id text NULL
);

CREATE INDEX IF NOT EXISTS idx_function_calls_fn_called
  ON function_calls (fn_name, called_at DESC);
CREATE INDEX IF NOT EXISTS idx_function_calls_workflow
  ON function_calls (workflow_id)
  WHERE workflow_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS sql_slow_queries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  query_text text NOT NULL,
  args_hash bytea NULL,
  duration_ms int NOT NULL,
  plan_text text NULL,
  workflow_id text NULL,
  captured_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sql_slow_queries_captured
  ON sql_slow_queries (captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_sql_slow_queries_duration
  ON sql_slow_queries (duration_ms DESC);

CREATE TABLE IF NOT EXISTS resource_samples (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  captured_at timestamptz NOT NULL DEFAULT now(),
  goroutines int NOT NULL,
  heap_alloc_bytes bigint NOT NULL,
  heap_sys_bytes bigint NOT NULL,
  num_gc int NOT NULL,
  num_cpu int NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_resource_samples_captured
  ON resource_samples (captured_at DESC);
