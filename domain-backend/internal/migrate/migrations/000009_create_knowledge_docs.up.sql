-- migration: create_knowledge_docs
-- author: mnunez@saargo.com
-- issue: HU-01.1 + HU-03.4
-- description: knowledge docs con chunking + embeddings + RAG-ready
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE knowledge_docs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  title VARCHAR(500) NOT NULL,
  body TEXT NOT NULL,
  body_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', body)) STORED,
  source VARCHAR(50) NOT NULL DEFAULT 'manual',
  source_url VARCHAR(1000),
  tags TEXT[] NOT NULL DEFAULT '{}',
  metadata JSONB NOT NULL DEFAULT '{}',
  has_attachments BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE knowledge_chunks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  knowledge_doc_id UUID NOT NULL REFERENCES knowledge_docs(id) ON DELETE CASCADE,
  chunk_index INT NOT NULL,
  content TEXT NOT NULL,
  content_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', content)) STORED,
  embedding vector(1536),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (knowledge_doc_id, chunk_index)
);

CREATE TRIGGER set_updated_at_knowledge_docs
  BEFORE UPDATE ON knowledge_docs
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX knowledge_docs_project_idx ON knowledge_docs (project_id)
  WHERE deleted_at IS NULL;
CREATE INDEX knowledge_docs_body_tsv_idx ON knowledge_docs USING GIN (body_tsv);
CREATE INDEX knowledge_chunks_doc_idx ON knowledge_chunks (knowledge_doc_id, chunk_index);
CREATE INDEX knowledge_chunks_embedding_idx ON knowledge_chunks
  USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX knowledge_chunks_content_tsv_idx ON knowledge_chunks USING GIN (content_tsv);
