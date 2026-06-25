






CREATE TABLE seed_versions (
  seeder_name VARCHAR(100) PRIMARY KEY,
  applied_version INT NOT NULL,
  last_applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_report JSONB
);
