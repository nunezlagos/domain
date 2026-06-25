






CREATE TABLE IF NOT EXISTS notification_deliveries (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  channel_slug    VARCHAR(40) NOT NULL,
  recipient       TEXT NOT NULL,
  template_slug   VARCHAR(80),
  subject         TEXT,
  body            TEXT NOT NULL,
  status          VARCHAR(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','sent','failed','retrying','dead')),
  attempt         INTEGER NOT NULL DEFAULT 1,
  response_code   INTEGER,
  error_message   TEXT,
  latency_ms      INTEGER,
  next_retry_at   TIMESTAMPTZ,
  delivered_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS notification_deliveries_org_idx
  ON notification_deliveries(organization_id, created_at DESC)
  WHERE organization_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS notification_deliveries_pending_idx
  ON notification_deliveries(next_retry_at)
  WHERE status IN ('pending','retrying');
CREATE INDEX IF NOT EXISTS notification_deliveries_channel_idx
  ON notification_deliveries(channel_slug, created_at DESC);

GRANT SELECT, INSERT, UPDATE ON notification_deliveries TO app_user;
GRANT SELECT ON notification_deliveries TO app_readonly;
