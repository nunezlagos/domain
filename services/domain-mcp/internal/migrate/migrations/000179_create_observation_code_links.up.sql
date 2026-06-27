-- migration: 000179_create_observation_code_links
-- author: NunezLagos
-- issue: REQ-knowledge-graph
-- description: cruce entre grafo de memoria (knowledge_observations) y grafo de codigo (code_nodes)
-- breaking: no
-- estimated_duration: unknown
--
-- detalle: CRUCE entre el grafo de MEMORIA (knowledge_observations) y el grafo
--   de CÓDIGO (code_nodes, mig 000178). Una decisión/memoria puede afectar, haberse
--   decidido sobre, referenciar o implementar un nodo de código concreto. Esta tabla
--   materializa ese vínculo dirigido observation -> code_node con un tipo semántico:
--     - affects     : la memoria/decisión impacta a ese nodo de código.
--     - decided_in  : la decisión se tomó en el contexto de ese nodo.
--     - references  : la memoria simplemente menciona/apunta a ese nodo.
--     - implements  : la memoria/decisión se implementa en ese nodo.
--
--   Permite preguntar en ambas direcciones:
--     - "¿qué decisiones afectan a esta función?" (por code_node_id).
--     - "¿qué nodos de código toca esta decisión?" (por observation_id).
--
--   org isolation: el modelo org fue retirado (mig 000142 dropeó organization_id de
--   todas las tablas; mig 000143 dropeó la tabla organizations). El sistema es
--   single-tenant: el aislamiento es por project_id, igual que las tablas que cruza.
--   Por eso esta tabla NO lleva organization_id ni FK a organizations. El project_id
--   debe coincidir con el de la observation y el del code_node (validado en la app).
-- nota: tabla nueva, sin backfill.

CREATE TABLE IF NOT EXISTS knowledge_observation_code_links (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  observation_id  UUID NOT NULL REFERENCES knowledge_observations(id) ON DELETE CASCADE,
  code_node_id    UUID NOT NULL REFERENCES code_nodes(id) ON DELETE CASCADE,

  link_type       VARCHAR(20) NOT NULL
    CONSTRAINT knowledge_observation_code_links_type_check
    CHECK (link_type IN ('affects','decided_in','references','implements')),

  note            TEXT,
  metadata        JSONB NOT NULL DEFAULT '{}',
  created_by      UUID REFERENCES users(id) ON DELETE SET NULL,

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);

-- un solo vínculo activo (no borrado) por (project, observation, code_node, tipo).
-- versiones borradas no chocan.
CREATE UNIQUE INDEX knowledge_observation_code_links_active_uniq
  ON knowledge_observation_code_links (project_id, observation_id, code_node_id, link_type)
  WHERE deleted_at IS NULL;

-- lookup "decisiones de esta memoria" / traversal desde la observation.
CREATE INDEX knowledge_observation_code_links_observation_idx
  ON knowledge_observation_code_links (observation_id)
  WHERE deleted_at IS NULL;

-- lookup "memorias que afectan este nodo" / traversal desde el code_node.
CREATE INDEX knowledge_observation_code_links_code_node_idx
  ON knowledge_observation_code_links (code_node_id)
  WHERE deleted_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON knowledge_observation_code_links TO app_user;
GRANT ALL ON knowledge_observation_code_links TO app_admin;
