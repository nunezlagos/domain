-- migration: create_captured_prompts
-- author: mnunez@saargo.com
-- issue: REQ-41 captura de prompts del usuario para análisis posterior
-- description: persistir cada mensaje del usuario al LLM (raw_text + sesión)
-- breaking: false
-- estimated_duration: <1s
--
-- Diferente a tabla `prompts` (templates reutilizables versionados).
-- Diferente a `intake_payloads` (requerimientos clasificados).
-- Acá guardamos el raw_text que el usuario escribió, sin filtro, para
-- después analizar: claridad, longitud, intent shifts, palabras
-- ambiguas, etc., y devolverle recomendaciones.

CREATE TABLE captured_prompts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
  project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  content TEXT NOT NULL,
  content_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', content)) STORED,
  -- metadata: cliente IDE (claude-code|opencode|cursor|...), modelo si se conoce
  client_kind VARCHAR(40),
  model VARCHAR(100),
  -- proxy de tokens (count de chars hasta tener integración real)
  char_count INT NOT NULL DEFAULT 0,
  captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX captured_prompts_org_session_idx ON captured_prompts(organization_id, session_id, captured_at DESC);
CREATE INDEX captured_prompts_org_user_idx ON captured_prompts(organization_id, user_id, captured_at DESC);
CREATE INDEX captured_prompts_org_project_idx ON captured_prompts(organization_id, project_id, captured_at DESC);
CREATE INDEX captured_prompts_tsv_idx ON captured_prompts USING gin(content_tsv);

-- RLS (defense-in-depth, patrón mig 000101+)
ALTER TABLE captured_prompts ENABLE ROW LEVEL SECURITY;
ALTER TABLE captured_prompts FORCE ROW LEVEL SECURITY;
CREATE POLICY captured_prompts_org_isolation ON captured_prompts
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON captured_prompts TO app_user;
GRANT ALL ON captured_prompts TO app_admin;
