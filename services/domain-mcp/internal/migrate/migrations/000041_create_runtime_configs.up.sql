






CREATE TABLE IF NOT EXISTS runtime_configs (
  key             VARCHAR(80) PRIMARY KEY,
  value           JSONB NOT NULL,
  description     TEXT NOT NULL DEFAULT '',
  is_hot_reloadable BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by      UUID
);

CREATE INDEX IF NOT EXISTS runtime_configs_updated_at_idx
  ON runtime_configs(updated_at DESC);

GRANT SELECT ON runtime_configs TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON runtime_configs TO app_admin;
GRANT SELECT ON runtime_configs TO app_readonly;



INSERT INTO runtime_configs (key, value, description, is_hot_reloadable) VALUES
  ('log_level', '"info"', 'slog level: debug|info|warn|error', TRUE),
  ('http_request_timeout_seconds', '30', 'Default timeout para outbound HTTP', TRUE),
  ('llm_default_timeout_seconds', '60', 'Default timeout para LLM calls', TRUE),
  ('otel_sample_ratio', '0.1', 'Sampling ratio para OpenTelemetry traces', TRUE),
  ('outbound_dispatcher_batch_size', '50', 'Batch size del dispatcher cada tick', TRUE)
ON CONFLICT (key) DO NOTHING;
