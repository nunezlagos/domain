# Design: issue-10.4-outbound-webhooks

## Decisión arquitectónica

**Event bus:** Postgres LISTEN/NOTIFY + tabla deliveries (mismo patrón que notifications issue-20).
**Signature:** HMAC SHA-256 con timestamp + body para anti-replay.
**Retry:** 8 attempts con backoff [10s,1m,5m,30m,2h,6h,12h,24h].
**Circuit breaker:** auto-pause si >50% 5xx en 1h.

## Schema

```sql
CREATE TABLE outbound_webhook_subscriptions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  name VARCHAR(255) NOT NULL,
  url TEXT NOT NULL,
  events TEXT[] NOT NULL,
  filters JSONB DEFAULT '{}',
  secret_encrypted BYTEA NOT NULL,   -- AES-256-GCM via issue-02.3
  enabled BOOLEAN DEFAULT true,
  paused_at TIMESTAMPTZ,
  paused_reason TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX ON outbound_webhook_subscriptions (organization_id, enabled);

CREATE TABLE outbound_webhook_deliveries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  subscription_id UUID NOT NULL REFERENCES outbound_webhook_subscriptions(id) ON DELETE CASCADE,
  event_type VARCHAR(100) NOT NULL,
  event_id UUID NOT NULL,
  payload JSONB NOT NULL,
  status VARCHAR(20) NOT NULL,   -- pending | sent | failed | dead
  attempt INT DEFAULT 0,
  next_attempt_at TIMESTAMPTZ,
  response_code INT,
  response_body_excerpt TEXT,
  latency_ms INT,
  error TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX ON outbound_webhook_deliveries (status, next_attempt_at) WHERE status IN ('pending','failed');
```

## Signature

```
sig_payload = timestamp + "." + body
signature = "sha256=" + hex(hmac_sha256(secret, sig_payload))
```

Receivers validate timestamp <5min skew + recompute HMAC.

## SSRF prevention

```go
// blocked: 127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, ::1, fd00::/8
// optional: allowlist hostnames in production env
```

## TDD plan

1. Subscribe + event → delivery
2. HMAC verifiable by receiver test
3. 503 retry 8x → DLQ
4. Filter matchea/no matchea
5. Test ping
6. SSRF intento → reject
7. Circuit breaker auto-pause
8. Replay deliverable
