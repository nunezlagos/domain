-- migration: create_budgets
-- author: nunezlagos
-- issue: issue-15.2
-- description: budgets de gasto LLM por org con threshold de warning
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS budgets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
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

-- domain-lint-ignore-next: require-concurrent-index-creation
-- reason: tabla nueva sin tráfico
CREATE INDEX IF NOT EXISTS budgets_org_idx
  ON budgets (organization_id) WHERE deleted_at IS NULL;

DO $trigger$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'set_updated_at_budgets'
  ) THEN
    CREATE TRIGGER set_updated_at_budgets
      BEFORE UPDATE ON budgets
      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END$trigger$;

GRANT SELECT, INSERT, UPDATE, DELETE ON budgets TO app_user;
