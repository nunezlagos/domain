-- migration: create_extensions
-- author: mnunez@saargo.com
-- issue: HU-01.1
-- description: extensiones core pgvector + pgcrypto + helper trigger set_updated_at
-- breaking: false
-- estimated_duration: <1s (empty DB)

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
