
CREATE TABLE IF NOT EXISTS org_delete_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    slug TEXT NOT NULL,
    actor_user_id UUID,
    actor_email TEXT,
    pre_counts JSONB NOT NULL DEFAULT '{}',
    s3_cleanup_failed BOOLEAN NOT NULL DEFAULT false,
    s3_configured BOOLEAN NOT NULL DEFAULT false,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
