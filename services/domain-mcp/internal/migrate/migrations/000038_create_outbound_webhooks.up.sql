-- migration: create_outbound_webhooks
-- author: nunezlagos
-- issue: HU-10.4
-- description: subscriptions + deliveries para webhooks outbound (notify URL on events)
-- breaking: false
-- estimated_duration: <1s

-- Subscriptions: URL + event types + filtros + secret cifrado
CREATE TABLE IF NOT EXISTS outbound_webhook_subscriptions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name            VARCHAR(80) NOT NULL,
  url             TEXT NOT NULL,
  events          TEXT[] NOT NULL,
  filters         JSONB NOT NULL DEFAULT '{}',
  secret_cipher   BYTEA,
  active          BOOLEAN NOT NULL DEFAULT TRUE,
  failure_count   INTEGER NOT NULL DEFAULT 0,
  last_success_at TIMESTAMPTZ,
  last_failure_at TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT outbound_webhook_subscriptions_url_check CHECK (url ~ '^https?://')
);

CREATE INDEX IF NOT EXISTS outbound_webhook_subscriptions_org_idx
  ON outbound_webhook_subscriptions(organization_id) WHERE active = TRUE;
CREATE INDEX IF NOT EXISTS outbound_webhook_subscriptions_events_gin
  ON outbound_webhook_subscriptions USING GIN(events);

CREATE TRIGGER set_updated_at_outbound_webhook_subscriptions
  BEFORE UPDATE ON outbound_webhook_subscriptions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Deliveries: una row por intento de delivery (audit completo)
CREATE TABLE IF NOT EXISTS outbound_webhook_deliveries (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  subscription_id  UUID NOT NULL REFERENCES outbound_webhook_subscriptions(id) ON DELETE CASCADE,
  organization_id  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  event_id         UUID NOT NULL,
  event_type       VARCHAR(80) NOT NULL,
  payload          JSONB NOT NULL,
  attempt          INTEGER NOT NULL DEFAULT 1,
  status           VARCHAR(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','succeeded','failed','dead_letter')),
  response_code    INTEGER,
  response_body    TEXT,
  duration_ms      INTEGER,
  error_message    TEXT,
  next_retry_at    TIMESTAMPTZ,
  delivered_at     TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS outbound_webhook_deliveries_sub_idx
  ON outbound_webhook_deliveries(subscription_id, created_at DESC);
CREATE INDEX IF NOT EXISTS outbound_webhook_deliveries_pending_idx
  ON outbound_webhook_deliveries(next_retry_at)
  WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS outbound_webhook_deliveries_org_idx
  ON outbound_webhook_deliveries(organization_id, created_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON outbound_webhook_subscriptions TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON outbound_webhook_deliveries TO app_user;
GRANT SELECT ON outbound_webhook_subscriptions TO app_readonly;
GRANT SELECT ON outbound_webhook_deliveries TO app_readonly;
