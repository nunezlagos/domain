CREATE TABLE org_cost_alert_thresholds (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id),
    daily_usd NUMERIC(10,2) NOT NULL DEFAULT 100.00,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE cost_alerts_sent (
    id BIGSERIAL PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    alert_date DATE NOT NULL,
    amount_usd NUMERIC(10,2) NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, alert_date)
);
