# Design: HU-20.1-channel-abstraction

## Decisión arquitectónica

**Cola:** tabla Postgres con SKIP LOCKED (no Redis/RabbitMQ — mantenemos Postgres-only).
**Templates:** `text/template` stdlib con `{{- if -}}`, strict mode con `Option("missingkey=error")`.
**Worker:** goroutine pool con context cancellation y graceful shutdown.

## Alternativas descartadas

- **Redis/RabbitMQ:** rompe principio Postgres-only de Domain
- **Sin templates DB, hardcoded:** dificulta i18n y A/B testing
- **Lib pkg/errors retry:** simple loop con backoff es suficiente

## Schema

```sql
CREATE TABLE notification_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug VARCHAR(100) NOT NULL,
  version INT NOT NULL,
  subject TEXT,
  body TEXT NOT NULL,
  variables JSONB NOT NULL DEFAULT '[]',
  organization_id UUID REFERENCES organizations(id),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE(organization_id, slug, version)
);

CREATE TABLE notification_subscriptions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id),
  user_id UUID REFERENCES users(id),
  event_type VARCHAR(100) NOT NULL,
  channel_slug VARCHAR(50) NOT NULL,
  recipient VARCHAR(500) NOT NULL,
  enabled BOOLEAN DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE notification_deliveries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id),
  channel_slug VARCHAR(50) NOT NULL,
  recipient VARCHAR(500) NOT NULL,
  template_slug VARCHAR(100),
  template_version INT,
  payload JSONB,           -- redactado, sin PII full
  status VARCHAR(20) NOT NULL,
  attempt INT DEFAULT 0,
  max_attempts INT DEFAULT 5,
  next_attempt_at TIMESTAMPTZ,
  latency_ms INT,
  response_code INT,
  error TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX ON notification_deliveries (status, next_attempt_at)
  WHERE status IN ('pending', 'retrying');
```

## Componentes

```
internal/notifications/
  channel.go        # interface + registry
  message.go        # Message struct
  template.go       # render
  worker.go         # pool processing pending deliveries
  retry.go          # backoff policy
  service.go        # Enqueue(event, payload)
```

## TDD plan

1. Registry register + get + concurrent safe
2. Template strict mode error en missing
3. Enqueue + worker procesa
4. Retry transitorio + dead letter
5. Subscription routing
