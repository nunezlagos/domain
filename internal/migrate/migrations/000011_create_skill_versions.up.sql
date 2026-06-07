-- migration: create_skill_versions
-- author: mnunez@saargo.com
-- issue: HU-01.1 + HU-05.3
-- description: snapshots immutables de skills por versión
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE skill_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  version INT NOT NULL,
  content TEXT,
  input_schema JSONB,
  output_schema JSONB,
  changelog TEXT,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (skill_id, version)
);

CREATE INDEX skill_versions_skill_idx ON skill_versions (skill_id, version DESC);
