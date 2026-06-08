-- migration: create_usage_alerts
-- author: nunezlagos
-- issue: HU-15.3
-- description: alerts configurables sobre métricas de cost/tokens + delivery + log
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS usage_alerts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name            VARCHAR(80) NOT NULL,
  metric          VARCHAR(40) NOT NULL,
  threshold       NUMERIC(14,4) NOT NULL CHECK (threshold >= 0),
  condition       VARCHAR(20) NOT NULL DEFAULT 'greater_than'
    CHECK (condition IN ('greater_than','less_than','equals')),
  channel         VARCHAR(20) NOT NULL DEFAULT 'webhook'
    CHECK (channel IN ('webhook','email','log_only')),
  recipients      TEXT[] NOT NULL DEFAULT '{}',
  cooldown_secs   INTEGER NOT NULL DEFAULT 3600,
  active          BOOLEAN NOT NULL DEFAULT TRUE,
  last_fired_at   TIMESTAMPTZ,
  fire_count      INTEGER NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS usage_alerts_org_active_idx
  ON usage_alerts(organization_id) WHERE active = TRUE;
CREATE INDEX IF NOT EXISTS usage_alerts_metric_idx
  ON usage_alerts(metric) WHERE active = TRUE;

CREATE TRIGGER set_updated_at_usage_alerts
  BEFORE UPDATE ON usage_alerts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Log de disparos para audit + debug.
CREATE TABLE IF NOT EXISTS usage_alert_fires (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  alert_id         UUID NOT NULL REFERENCES usage_alerts(id) ON DELETE CASCADE,
  organization_id  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  metric           VARCHAR(40) NOT NULL,
  threshold        NUMERIC(14,4) NOT NULL,
  observed_value   NUMERIC(14,4) NOT NULL,
  payload          JSONB NOT NULL DEFAULT '{}',
  fired_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS usage_alert_fires_alert_idx
  ON usage_alert_fires(alert_id, fired_at DESC);
CREATE INDEX IF NOT EXISTS usage_alert_fires_org_idx
  ON usage_alert_fires(organization_id, fired_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON usage_alerts TO app_user;
GRANT SELECT, INSERT ON usage_alert_fires TO app_user;
GRANT SELECT ON usage_alerts TO app_readonly;
GRANT SELECT ON usage_alert_fires TO app_readonly;
