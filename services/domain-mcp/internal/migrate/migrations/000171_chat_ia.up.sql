-- migration: 000171_chat_ia
-- description: tablas para el chat IA estilo NotebookLM (HU-49.2).
--   3 tablas: chat_conversations, chat_messages, chat_document_embeddings.
--   Persisten conversaciones del admin + embeddings para RAG sobre las
--   tablas del dominio (agents, skills, flows, prompts, projects, etc).
-- breaking: no (tablas nuevas, sin tocar esquema existente).
-- scope: domain-mcp. domain-admin (Django) las consume con managed=False.
-- RLS: pendiente HU-49.5. Hoy single-org, app_user tiene acceso libre.

-- ============================================================
-- chat_conversations: una conversacion por usuario
-- ============================================================
CREATE TABLE chat_conversations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_email  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX chat_conversations_user_idx
    ON chat_conversations (user_email, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_chat_conversations
    BEFORE UPDATE ON chat_conversations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- chat_messages: mensajes de una conversacion
--   user    -> pregunta
--   assistant -> respuesta del LLM (con status pending/processing/completed/error)
-- ============================================================
CREATE TABLE chat_messages (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content         TEXT,
    content_partial TEXT,
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'processing', 'completed', 'error')),
    sources         JSONB NOT NULL DEFAULT '[]'::jsonb,
    tokens_in       INTEGER NOT NULL DEFAULT 0,
    tokens_out      INTEGER NOT NULL DEFAULT 0,
    model           TEXT NOT NULL DEFAULT '',
    duration_ms     INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX chat_messages_conv_idx
    ON chat_messages (conversation_id, created_at DESC);

-- ============================================================
-- chat_document_embeddings: cache de embeddings para RAG live-read
--   source_table: nombre de la tabla origen (agents, skills, etc).
--   source_id:    PK de la fila origen.
--   source_url:   URL del admin para ver el detalle (/agentes/detalle/<id>).
--   chunk_text:   texto del chunk embebido.
--   chunk_index:  ordinal del chunk dentro del row (0..N-1).
--   embedding:    vector(1536) para text-embedding-3-small.
--   valid_until:  cuando expira el cache. NULL = permanente.
-- ============================================================
CREATE TABLE chat_document_embeddings (
    id           BIGSERIAL PRIMARY KEY,
    source_table TEXT NOT NULL,
    source_id    UUID NOT NULL,
    source_url   TEXT NOT NULL DEFAULT '',
    chunk_text   TEXT NOT NULL,
    chunk_index  INTEGER NOT NULL DEFAULT 0,
    embedding    vector(1536) NOT NULL,
    model        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until  TIMESTAMPTZ,
    UNIQUE (source_table, source_id, chunk_index)
);
CREATE INDEX chat_doc_emb_source_idx
    ON chat_document_embeddings (source_table, source_id);
CREATE INDEX chat_doc_emb_vec_idx
    ON chat_document_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
CREATE INDEX chat_doc_emb_valid_idx
    ON chat_document_embeddings (valid_until)
    WHERE valid_until IS NOT NULL;

-- ============================================================
-- Grants (mismo patron que el resto de las tablas)
-- ============================================================
GRANT SELECT, INSERT, UPDATE, DELETE ON chat_conversations TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON chat_messages TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON chat_document_embeddings TO app_user;
GRANT USAGE, SELECT ON SEQUENCE chat_messages_id_seq TO app_user;
GRANT USAGE, SELECT ON SEQUENCE chat_document_embeddings_id_seq TO app_user;
GRANT ALL ON chat_conversations, chat_messages, chat_document_embeddings TO app_admin;
GRANT USAGE, SELECT ON SEQUENCE chat_messages_id_seq TO app_admin;
GRANT USAGE, SELECT ON SEQUENCE chat_document_embeddings_id_seq TO app_admin;
