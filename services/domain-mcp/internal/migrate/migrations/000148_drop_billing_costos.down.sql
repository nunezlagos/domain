




























CREATE TABLE IF NOT EXISTS plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug VARCHAR(50) NOT NULL UNIQUE,
  name VARCHAR(100) NOT NULL,
  tokens_per_month BIGINT,
  runs_per_month INTEGER,
  storage_gb_max INTEGER,
  members_max INTEGER,
  seats INTEGER,
  soft_limit_ratio NUMERIC(3,2) NOT NULL DEFAULT 0.80
    CHECK (soft_limit_ratio > 0 AND soft_limit_ratio <= 1.0),
  monthly_price_usd NUMERIC(10,2) NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS budgets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(120) NOT NULL,
  amount_usd NUMERIC(12,4) NOT NULL CHECK (amount_usd > 0),
  period VARCHAR(10) NOT NULL DEFAULT 'monthly'
    CHECK (period IN ('daily','weekly','monthly')),
  warning_threshold_pct INT NOT NULL DEFAULT 80
    CHECK (warning_threshold_pct BETWEEN 1 AND 100),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS cost_logs (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  flow_run_id UUID REFERENCES flow_runs(id) ON DELETE SET NULL,
  agent_run_id UUID REFERENCES agent_runs(id) ON DELETE SET NULL,
  provider VARCHAR(50) NOT NULL,
  model VARCHAR(100) NOT NULL,
  operation VARCHAR(30) NOT NULL,
  tokens_input BIGINT NOT NULL DEFAULT 0,
  tokens_output BIGINT NOT NULL DEFAULT 0,
  tokens_cached BIGINT NOT NULL DEFAULT 0,
  cost_usd NUMERIC(12,6) NOT NULL,
  latency_ms INT,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (operation IN ('completion', 'embedding', 'image', 'audio', 'tool_call'))
);

CREATE TABLE IF NOT EXISTS org_cost_alert_thresholds (
  id BIGSERIAL PRIMARY KEY,
  daily_usd NUMERIC(10,2) NOT NULL DEFAULT 100.00,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cost_alerts_sent (
  id BIGSERIAL PRIMARY KEY,
  alert_date DATE NOT NULL UNIQUE,
  amount_usd NUMERIC(10,2) NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
