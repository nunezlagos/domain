-- ============================================================================
-- 01-extensions.sql — extensiones requeridas por domain.
-- Ejecutado por docker-entrypoint-initdb.d en el primer boot del container.
-- Idempotente (IF NOT EXISTS).
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS vector;       -- pgvector (embeddings REQ-06)
