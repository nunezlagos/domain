






CREATE TABLE IF NOT EXISTS platform_policies (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug            VARCHAR(80) NOT NULL,
  name            VARCHAR(160) NOT NULL,
  kind            VARCHAR(40) NOT NULL
    CHECK (kind IN (
      'convention','security_rule','architecture','sdd_workflow',
      'observability','migration_rule','linter_config'
    )),
  body_md         TEXT NOT NULL,
  body_structured JSONB NOT NULL DEFAULT '{}',
  version         INTEGER NOT NULL DEFAULT 1,
  is_active       BOOLEAN NOT NULL DEFAULT TRUE,
  source_file     VARCHAR(120),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT platform_policies_slug_active_unique UNIQUE (slug, is_active)
);

CREATE INDEX IF NOT EXISTS platform_policies_kind_idx
  ON platform_policies(kind) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS platform_policies_slug_idx
  ON platform_policies(slug);

CREATE TRIGGER set_updated_at_platform_policies
  BEFORE UPDATE ON platform_policies
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


CREATE TABLE IF NOT EXISTS platform_policy_versions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  policy_id       UUID NOT NULL REFERENCES platform_policies(id) ON DELETE CASCADE,
  version         INTEGER NOT NULL,
  body_md         TEXT NOT NULL,
  body_structured JSONB NOT NULL DEFAULT '{}',
  changed_by      UUID,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT platform_policy_versions_unique UNIQUE (policy_id, version)
);

CREATE INDEX IF NOT EXISTS platform_policy_versions_policy_idx
  ON platform_policy_versions(policy_id, version DESC);

GRANT SELECT ON platform_policies TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON platform_policies TO app_admin;
GRANT SELECT ON platform_policy_versions TO app_user;
GRANT SELECT, INSERT ON platform_policy_versions TO app_admin;
GRANT SELECT ON platform_policies TO app_readonly;
GRANT SELECT ON platform_policy_versions TO app_readonly;
