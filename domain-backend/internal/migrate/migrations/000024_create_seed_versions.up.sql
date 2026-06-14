-- migration: create_seed_versions
-- author: mnunez@saargo.com
-- issue: HU-01.7
-- description: tabla seed_versions tracking aplicación de seeders idempotentes
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE seed_versions (
  seeder_name VARCHAR(100) PRIMARY KEY,
  applied_version INT NOT NULL,
  last_applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_report JSONB
);
