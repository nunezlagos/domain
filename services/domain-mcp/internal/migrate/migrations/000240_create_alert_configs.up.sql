CREATE TABLE IF NOT EXISTS alert_configs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  category text NOT NULL,
  severity_min text NOT NULL DEFAULT 'error'
    CHECK (severity_min IN ('debug','info','warn','error','critical')),
  channel text NOT NULL
    CHECK (channel IN ('webhook','email','ntfy','log_only')),
  channel_config jsonb NOT NULL DEFAULT '{}',
  throttle_seconds int NOT NULL DEFAULT 60,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alert_configs_category
  ON alert_configs (category)
  WHERE enabled;
