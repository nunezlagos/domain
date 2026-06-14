CREATE TABLE org_flow_config (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id),
    max_flow_duration_seconds INT NOT NULL DEFAULT 300
        CHECK (max_flow_duration_seconds >= 10 AND max_flow_duration_seconds <= 86400),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
