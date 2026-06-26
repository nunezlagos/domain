-- migration: 000171_chat_ia (down)
-- description: rollback de las 3 tablas del chat IA.

DROP TABLE IF EXISTS chat_document_embeddings CASCADE;
DROP TABLE IF EXISTS chat_messages CASCADE;
DROP TABLE IF EXISTS chat_conversations CASCADE;
