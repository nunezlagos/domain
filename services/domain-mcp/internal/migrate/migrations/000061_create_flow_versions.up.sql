-- migration: create_flow_versions
-- author: nunezlagos
-- issue: HU-09.7
-- description: snapshots immutables de flow definitions por versión
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE flow_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_id UUID NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
  version INT NOT NULL,
  definition JSONB NOT NULL,
  hash VARCHAR(64) NOT NULL,        -- SHA-256 hex de definition normalizado
  note TEXT,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(flow_id, version),
  UNIQUE(flow_id, hash)             -- idempotencia: mismo hash = no-op
);

CREATE INDEX flow_versions_flow_idx ON flow_versions (flow_id, version DESC);

GRANT SELECT, INSERT ON flow_versions TO app_user;
