# Proposal: HU-03.4-knowledge-documents

## Intención

Implementar almacenamiento de documentos de conocimiento largos con chunking automático y embeddings vectoriales usando pgvector. Esto permite búsqueda semántica (RAG) sobre el conocimiento persistente del sistema, diferenciándose de las observaciones que son entradas cortas y estructuradas.

## Scope

**Incluye:**
- Tabla `knowledge_chunks` con: `id` (UUID), `document_id` (UUID, grouping key), `title`, `content` (TEXT), `chunk_index` (INT), `embedding` (VECTOR, dim configurable), `project`, `created_at`, `updated_at`
- Tabla `knowledge_documents` (metadata): `id`, `title`, `project`, `created_at`, `updated_at`
- Chunking strategy: recursive character text splitter (similar a LangChain) con chunk_size=512, chunk_overlap=50
- Embedding: interfaz `Embedder` con método `Embed(texts []string) ([][]float32, error)`, implementación vía API (REQ-06)
- Semantic search: `SELECT * FROM knowledge_chunks ORDER BY embedding <=> $1 LIMIT $2` (cosine distance)
- Hybrid search: `ts_rank(tsv, query) * 0.3 + (1 - (embedding <=> $1)) * 0.7` como ranking combinado
- CRUD básico sobre documentos y chunks

**Excluye:**
- El servicio de embeddings en sí (REQ-06)
- Interfaz de usuario para gestión de documentos
- RAG pipeline completo (solo almacenamiento y búsqueda)

## Enfoque técnico

1. **Migración**: `CREATE EXTENSION IF NOT EXISTS vector`, luego tablas
2. **Chunker**: struct `RecursiveCharacterTextSplitter` con `chunk_size`, `chunk_overlap`, `separators []string`
3. **Embedder**: interfaz, implementación HTTP call al servicio de embeddings
4. **Store**: `KnowledgeStore` con métodos para documentos y chunks
5. **Índice**: IVFFlat o HNSW sobre el vector embedding (según tamaño)

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| pgvector no disponible | Alto | Feature detection al iniciar; error claro si no está instalado |
| Embedding API lenta | Medio | Cache de embeddings, batch de chunks |
| Dimensión de embedding incorrecta | Medio | Configurable, validar al crear índice |
| Chunking rompe contexto semántico | Medio | Usar chunk_overlap para mantener coherencia entre chunks |

## Testing

- **Unitarios**: chunker con textos de prueba, mock embedder
- **Integración**: pgvector container, insertar chunks con vectores, semantic search
- **Regression**: hybrid search con diferentes pesos
