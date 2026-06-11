# Tasks: issue-03.4-knowledge-documents

## Backend

- [x] `migrations/XXXX_enable_pgvector.sql`: `CREATE EXTENSION IF NOT EXISTS vector`
- [x] `migrations/XXXX_create_knowledge_documents.sql`: tabla knowledge_documents + knowledge_chunks + índices (IVFFlat, GIN tsvector, BTREE)
- [x] `internal/rag/chunker.go`: `RecursiveCharacterTextSplitter` struct con Split(text) ([]Chunk, error)
- [x] `internal/rag/chunker_test.go`: probar chunking con textos de 100, 1000, 5000 tokens
- [x] `internal/rag/embedder.go`: interfaz `Embedder` con `Embed(texts []string) ([][]float32, error)`
- [x] Implementar embedder HTTP client (apunta a REQ-06)
- [x] `internal/store/pg/knowledge.go`: interfaz `KnowledgeStore` + structs
- [x] Implementar `CreateDocument(doc) (uuid.UUID, error)`
- [x] Implementar `InsertChunks(chunks []Chunk) error`
- [x] Implementar `SemanticSearch(query vector, project string, limit int) ([]ChunkSearchResult, error)`
- [x] Implementar `HybridSearch(vector vector, text string, project string, limit int) ([]ChunkSearchResult, error)`
- [x] Implementar `GetDocumentChunks(documentID uuid.UUID) ([]Chunk, error)`
- [x] Implementar `DeleteDocument(documentID uuid.UUID) error`
- [x] `internal/memory/service.go`: integrar documento + chunking + embedding

## Tests

- [x] Test de chunker: texto de 1000 chars → ~2 chunks de 512, overlap preservado
- [x] Test de chunker: separadores en orden correcto
- [x] Test de integración: crear documento → buscar semánticamente → encontrar chunks del documento
- [x] Test de integración: hybrid search con peso combinado
- [x] Test de paginación en search
- [x] Sabotaje: pgvector no disponible → error claro al migrar

## Cierre

- [x] Verificación manual: crear documento con 2000 tokens, buscar semánticamente, confirmar ranking
- [x] Suite verde
