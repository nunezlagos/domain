-- migration: create_llm_semantic_cache
-- author: nunezlagos
-- issue: HU-07.3
-- description: cache semántico de respuestas LLM por embedding similarity
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE llm_semantic_cache (
  id VARCHAR(64) PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  provider VARCHAR(40) NOT NULL,
  model VARCHAR(80) NOT NULL,
  params_hash VARCHAR(64) NOT NULL,        -- SHA-256 hex de temperature/top_p/etc
  prompt_hash VARCHAR(64) NOT NULL,        -- SHA-256 hex del prompt exact
  prompt_preview TEXT NOT NULL,            -- primeras 200 chars para debug
  response JSONB NOT NULL,
  tokens INT NOT NULL DEFAULT 0,
  hit_count INT NOT NULL DEFAULT 0,
  prompt_embedding vector(1536) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(organization_id, provider, model, params_hash, prompt_hash)
);

CREATE INDEX llm_semantic_cache_embedding_idx
  ON llm_semantic_cache USING ivfflat (prompt_embedding vector_cosine_ops)
  WITH (lists = 100);

CREATE INDEX llm_semantic_cache_filter_idx
  ON llm_semantic_cache (organization_id, provider, model, params_hash, last_used_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON llm_semantic_cache TO app_user;
