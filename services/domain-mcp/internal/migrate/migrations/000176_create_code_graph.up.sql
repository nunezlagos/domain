-- migration: 000176_create_code_graph
-- description: grafo de CÓDIGO del repo (Go-only v1) persistido en Postgres.
--   El binario domain-mcp stdio corre client-side, parsea el AST del filesystem
--   y materializa el grafo en estas tablas para luego cruzarlo con las memorias
--   (knowledge_observations). Tres tablas:
--     - code_nodes        : entidades del código (file/package/func/method/type/
--                           interface/const/var) con su ubicación, firma y doc.
--     - code_edges        : relaciones tipadas entre nodos (calls/imports/
--                           implements/references/defined_in/method_of).
--     - code_index_files  : control de incrementalidad por archivo (content_hash +
--                           git_head), espejando el patrón project_index_runs.
--
--   INCREMENTALIDAD: code_index_files.content_hash permite saltar archivos no
--   cambiados; al re-parsear un archivo se hace SoftDelete de sus nodos y se
--   borran las aristas salientes antes de re-insertar.
--
--   org isolation: el modelo org fue retirado (mig 000142 dropeó organization_id
--   de todas las tablas; mig 000143 dropeó la tabla organizations). El sistema es
--   single-tenant: el aislamiento es por project_id. Estas tablas NO llevan
--   organization_id ni FK a organizations.
-- breaking: no (tablas nuevas, sin backfill).

-- ===========================================================================
-- code_nodes — entidades del grafo de código
-- ===========================================================================
CREATE TABLE IF NOT EXISTS code_nodes (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

  kind            VARCHAR(20) NOT NULL
    CONSTRAINT code_nodes_kind_check
    CHECK (kind IN ('file','package','func','method','type','interface','const','var')),

  name            TEXT,
  -- nombre cualificado único dentro del project: 'pkg.Func' o 'pkg.Type.Method'.
  qualified_name  TEXT,
  file_path       TEXT,
  line_start      INT,
  line_end        INT,
  signature       TEXT,
  doc             TEXT,
  language        VARCHAR(20) NOT NULL DEFAULT 'go',
  -- hash del fragmento (para detectar cambios a nivel nodo).
  content_hash    BYTEA,

  metadata        JSONB NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);

CREATE TRIGGER set_updated_at_code_nodes
  BEFORE UPDATE ON code_nodes
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- un solo nodo activo por (project, qualified_name, kind); versiones borradas no chocan.
CREATE UNIQUE INDEX code_nodes_qualified_uniq
  ON code_nodes (project_id, qualified_name, kind)
  WHERE deleted_at IS NULL;

-- lookup por archivo (re-parseo / SoftDelete por archivo).
CREATE INDEX code_nodes_file_idx
  ON code_nodes (project_id, file_path)
  WHERE deleted_at IS NULL;

-- filtrar por tipo de nodo dentro de un project.
CREATE INDEX code_nodes_kind_idx
  ON code_nodes (project_id, kind)
  WHERE deleted_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON code_nodes TO app_user;
GRANT ALL ON code_nodes TO app_admin;

-- ===========================================================================
-- code_edges — relaciones tipadas entre nodos
-- ===========================================================================
CREATE TABLE IF NOT EXISTS code_edges (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  source_node_id  UUID NOT NULL REFERENCES code_nodes(id) ON DELETE CASCADE,
  target_node_id  UUID NOT NULL REFERENCES code_nodes(id) ON DELETE CASCADE,

  edge_type       VARCHAR(20) NOT NULL
    CONSTRAINT code_edges_type_check
    CHECK (edge_type IN ('calls','imports','implements','references','defined_in','method_of')),

  metadata        JSONB DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT code_edges_no_self CHECK (source_node_id <> target_node_id)
);

-- una sola arista por (par dirigido + tipo) dentro del project.
CREATE UNIQUE INDEX code_edges_uniq
  ON code_edges (project_id, source_node_id, target_node_id, edge_type);

-- traversal forward (vecinos salientes de un nodo).
CREATE INDEX code_edges_source_idx
  ON code_edges (project_id, source_node_id);

-- traversal backward (quién apunta a este nodo).
CREATE INDEX code_edges_target_idx
  ON code_edges (project_id, target_node_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON code_edges TO app_user;
GRANT ALL ON code_edges TO app_admin;

-- ===========================================================================
-- code_index_files — control de incrementalidad por archivo
-- ===========================================================================
CREATE TABLE IF NOT EXISTS code_index_files (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  file_path       TEXT NOT NULL,
  content_hash    BYTEA NOT NULL,
  git_head        VARCHAR(40),
  node_count      INT DEFAULT 0,
  indexed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX code_index_files_uniq
  ON code_index_files (project_id, file_path);

GRANT SELECT, INSERT, UPDATE, DELETE ON code_index_files TO app_user;
GRANT ALL ON code_index_files TO app_admin;
