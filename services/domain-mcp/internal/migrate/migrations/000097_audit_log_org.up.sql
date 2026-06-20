ALTER TABLE audit_log ADD COLUMN origin_org_id UUID REFERENCES organizations(id);
CREATE INDEX idx_audit_log_org_time ON audit_log(origin_org_id, occurred_at DESC);
CREATE INDEX idx_audit_log_org_action ON audit_log(origin_org_id, action) WHERE origin_org_id IS NOT NULL;
