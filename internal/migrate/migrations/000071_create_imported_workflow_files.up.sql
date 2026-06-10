-- migration: create_imported_workflow_files
-- author: nunezlagos
-- issue: HU-12.7 (workflow-override) — Domain MCP reemplaza .md de Claude
--                Code / OpenCode / Cursor / etc., guardando backup en BD.
-- description: registry de archivos .md de instrucciones IA importados
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE imported_workflow_files (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  source_tool VARCHAR(40) NOT NULL
    CHECK (source_tool IN ('claude-code','opencode','cursor','windsurf','aider','generic')),
  -- Path relativo desde el root del proyecto donde se encontró.
  rel_path TEXT NOT NULL,
  original_content TEXT NOT NULL,       -- backup completo para restore
  content_hash VARCHAR(64) NOT NULL,    -- SHA-256 hex del original
  size_bytes BIGINT NOT NULL,
  -- Estado: detected → backed_up → replaced (con stub) → restored (rollback)
  status VARCHAR(20) NOT NULL DEFAULT 'detected'
    CHECK (status IN ('detected','backed_up','replaced','restored')),
  replaced_with TEXT,                    -- content del stub que escribimos
  replaced_at TIMESTAMPTZ,
  restored_at TIMESTAMPTZ,
  metadata JSONB NOT NULL DEFAULT '{}',  -- ej: detected_at, file_mode, etc.
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT imported_workflow_files_unique UNIQUE (project_id, rel_path)
);

CREATE INDEX imported_workflow_files_project_idx
  ON imported_workflow_files (project_id, status);
CREATE INDEX imported_workflow_files_tool_idx
  ON imported_workflow_files (source_tool, status);

CREATE TRIGGER set_updated_at_imported_workflow_files
  BEFORE UPDATE ON imported_workflow_files
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON imported_workflow_files TO app_user;
