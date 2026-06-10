# Tasks: issue-03.4-knowledge-documents

## Backend

- [ ] `migrations/XXXX_enable_pgvector.sql`: `CREATE EXTENSION IF NOT EXISTS vector`
- [ ] `migrations/XXXX_create_knowledge_documents.sql`: tabla knowledge_documents + knowledge_chunks + índices (IVFFlat, GIN tsvector, BTREE)
- [ ] `internal/rag/chunker.go`: `RecursiveCharacterTextSplitter` struct con Split(text) ([]Chunk, error)
- [ ] `internal/rag/chunker_test.go`: probar chunking con textos de 100, 1000, 5000 tokens
- [ ] `internal/rag/embedder.go`: interfaz `Embedder` con `Embed(texts []string) ([][]float32, error)`
- [ ] Implementar embedder HTTP client (apunta a REQ-06)
- [ ] `internal/store/pg/knowledge.go`: interfaz `KnowledgeStore` + structs
- [ ] Implementar `CreateDocument(doc) (uuid.UUID, error)`
- [ ] Implementar `InsertChunks(chunks []Chunk) error`
- [ ] Implementar `SemanticSearch(query vector, project string, limit int) ([]ChunkSearchResult, error)`
- [ ] Implementar `HybridSearch(vector vector, text string, project string, limit int) ([]ChunkSearchResult, error)`
- [ ] Implementar `GetDocumentChunks(documentID uuid.UUID) ([]Chunk, error)`
- [ ] Implementar `DeleteDocument(documentID uuid.UUID) error`
- [ ] `internal/memory/service.go`: integrar documento + chunking + embedding

## Tests

- [ ] Test de chunker: texto de 1000 chars → ~2 chunks de 512, overlap preservado
- [ ] Test de chunker: separadores en orden correcto
- [ ] Test de integración: crear documento → buscar semánticamente → encontrar chunks del documento
- [ ] Test de integración: hybrid search con peso combinado
- [ ] Test de paginación en search
- [ ] Sabotaje: pgvector no disponible → error claro al migrar

## Cierre

- [ ] Verificación manual: crear documento con 2000 tokens, buscar semánticamente, confirmar ranking
- [ ] Suite verde
