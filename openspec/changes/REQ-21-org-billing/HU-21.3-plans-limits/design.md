# Design: HU-21.3-plans-limits

## Schema

```sql
CREATE TABLE plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug VARCHAR(50) UNIQUE NOT NULL,  -- free, pro, enterprise
  name VARCHAR(100) NOT NULL,
  price_usd_monthly NUMERIC(10,2),
  limits JSONB NOT NULL,
  -- {"tokens_per_month": 100000, "runs_per_month": 100, "storage_gb": 1, "members": 3, "seats": 1}
  created_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE organizations ADD COLUMN plan_id UUID REFERENCES plans(id);
ALTER TABLE organizations ADD COLUMN custom_limits JSONB DEFAULT '{}';

CREATE TABLE usage_counters (
  organization_id UUID NOT NULL REFERENCES organizations(id),
  dimension VARCHAR(50) NOT NULL, -- tokens, runs
  period DATE NOT NULL,           -- first day of month
  amount BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (organization_id, dimension, period)
) PARTITION BY RANGE (period);
```

## Servicio quota

```go
type Quota interface {
  Check(ctx, orgID, dimension string, estimated int64) error
  Record(ctx, orgID, dimension string, actual int64) error
}
```

## TDD plan

1. Free plan llena 80% → notif soft
2. Llena 100% → Check returns ErrQuotaExceeded
3. Reset cron → counters 0 al 1ro
4. Custom limits 10M > free 100k → respeta custom
5. Race: 100 goroutines record 1 token → final = 100
