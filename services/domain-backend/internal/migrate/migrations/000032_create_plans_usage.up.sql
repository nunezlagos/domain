-- migration: create_plans_usage
-- author: mnunez@saargo.com
-- issue: HU-21.3
-- description: planes + usage tracking + custom_limits override per org
-- breaking: false
-- estimated_duration: <1s

-- Planes globales (no scoped por org)
CREATE TABLE plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug VARCHAR(50) NOT NULL UNIQUE,
  name VARCHAR(100) NOT NULL,
  -- Límites: NULL = ilimitado. INT64 para tokens (puede ser grande).
  tokens_per_month BIGINT,
  runs_per_month INTEGER,
  storage_gb_max INTEGER,
  members_max INTEGER,
  seats INTEGER,
  -- Soft limit warning threshold (default 0.8 = 80%)
  soft_limit_ratio NUMERIC(3,2) NOT NULL DEFAULT 0.80
    CHECK (soft_limit_ratio > 0 AND soft_limit_ratio <= 1.0),
  monthly_price_usd NUMERIC(10,2) NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER set_updated_at_plans
  BEFORE UPDATE ON plans
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Asociación org → plan + custom_limits override
ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS plan_id UUID REFERENCES plans(id) ON DELETE RESTRICT,
  ADD COLUMN IF NOT EXISTS custom_limits JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS plan_started_at TIMESTAMPTZ;

-- Usage counters per (org, period)
-- period_start = primer día del mes UTC; un row por (org, mes).
CREATE TABLE usage_counters (
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  period_start DATE NOT NULL,
  tokens_used BIGINT NOT NULL DEFAULT 0,
  runs_count INTEGER NOT NULL DEFAULT 0,
  storage_bytes BIGINT NOT NULL DEFAULT 0,
  cost_usd NUMERIC(12,4) NOT NULL DEFAULT 0,
  warned_80pct BOOLEAN NOT NULL DEFAULT false,
  warned_100pct BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (organization_id, period_start)
);

CREATE INDEX usage_counters_period_idx ON usage_counters (period_start);

CREATE TRIGGER set_updated_at_usage_counters
  BEFORE UPDATE ON usage_counters
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

GRANT SELECT, INSERT, UPDATE ON plans TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON usage_counters TO app_user;
GRANT ALL ON plans, usage_counters TO app_admin;

-- Seed planes default (free, pro, enterprise)
INSERT INTO plans (slug, name, tokens_per_month, runs_per_month, storage_gb_max, members_max, seats, monthly_price_usd)
VALUES
  ('free',       'Free',       100000,    100,    1,   3,  1,   0.00),
  ('pro',        'Pro',        5000000,   5000,   50,  25, 10,  49.00),
  ('enterprise', 'Enterprise', NULL,      NULL,   500, NULL, NULL, 499.00)
ON CONFLICT (slug) DO NOTHING;
