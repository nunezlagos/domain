






CREATE TABLE model_registry (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider VARCHAR(50) NOT NULL
    CHECK (provider IN ('anthropic','openai','google','ollama','voyage')),
  model VARCHAR(100) NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  modality VARCHAR(20) NOT NULL DEFAULT 'completion'
    CHECK (modality IN ('completion','embedding','image','audio')),
  context_size INT,

  input_per_million NUMERIC(10,4),
  output_per_million NUMERIC(10,4),

  embedding_dimensions INT,
  is_active BOOLEAN NOT NULL DEFAULT true,
  deprecated_at TIMESTAMPTZ,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (provider, model)
);

CREATE TRIGGER set_updated_at_model_registry
  BEFORE UPDATE ON model_registry
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX model_registry_provider_active_idx
  ON model_registry (provider, model) WHERE is_active = true;

GRANT SELECT ON model_registry TO app_user, app_readonly;
GRANT ALL ON model_registry TO app_admin;



INSERT INTO model_registry
  (provider, model, display_name, modality, context_size,
   input_per_million, output_per_million, embedding_dimensions, notes)
VALUES

  ('anthropic', 'claude-opus-4-7', 'Claude Opus 4.7', 'completion', 200000, 15.00, 75.00, NULL, NULL),
  ('anthropic', 'claude-sonnet-4-6', 'Claude Sonnet 4.6', 'completion', 200000, 3.00, 15.00, NULL, NULL),
  ('anthropic', 'claude-haiku-4-5', 'Claude Haiku 4.5', 'completion', 200000, 0.80, 4.00, NULL, NULL),

  ('openai', 'gpt-4o', 'GPT-4o', 'completion', 128000, 2.50, 10.00, NULL, NULL),
  ('openai', 'gpt-4o-mini', 'GPT-4o mini', 'completion', 128000, 0.15, 0.60, NULL, NULL),
  ('openai', 'text-embedding-3-small', 'OpenAI Embedding small', 'embedding', NULL, 0.02, NULL, 1536, NULL),
  ('openai', 'text-embedding-3-large', 'OpenAI Embedding large', 'embedding', NULL, 0.13, NULL, 3072, NULL),

  ('voyage', 'voyage-3', 'Voyage 3', 'embedding', NULL, 0.06, NULL, 1024, NULL),

  ('ollama', 'llama3.1', 'Llama 3.1 (local)', 'completion', 128000, 0, 0, NULL, 'local; sin costo'),
  ('ollama', 'nomic-embed-text', 'Nomic embed (local)', 'embedding', NULL, 0, NULL, 768, 'local')
ON CONFLICT (provider, model) DO NOTHING;
