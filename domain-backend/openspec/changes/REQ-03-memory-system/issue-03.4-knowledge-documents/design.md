# Design: issue-03.4-knowledge-documents

## Decisión arquitectónica

**Dos tablas (documentos metadata + chunks con embeddings) + chunker configurable + índice vectorial.**

```
knowledge_documents
├── id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── title       VARCHAR(500) NOT NULL
├── project     VARCHAR(255) NOT NULL
├── created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()

knowledge_chunks
├── id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── document_id     UUID NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE
├── title           VARCHAR(500) NOT NULL      -- heredado del documento
├── content         TEXT NOT NULL
├── chunk_index     INT NOT NULL
├── embedding       VECTOR(1536)               -- dimensión por defecto (text-embedding-3-small)
├── tsv             TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', content)) STORED
├── project         VARCHAR(255) NOT NULL
├── created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Índices:**
- `chunks_embedding_idx` IVFFlat (embedding) con listas = sqrt(n_registros)
- `chunks_document_id_idx` BTREE (document_id)
- `chunks_project_idx` BTREE (project)
- `chunks_tsv_idx` GIN (tsv) — para hybrid search

**Chunker:**
```
RecursiveCharacterTextSplitter:
  chunk_size: 512
  chunk_overlap: 50
  separators: ["\n\n", "\n", ".", " ", ""]
```
Divide recursivamente por separadores hasta que cada chunk sea <= chunk_size.

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Un solo documento con todo el contenido | No permite búsqueda granular ni RAG efectivo |
| Chunking solo por tokens fijos | Los separadores semánticos dan mejor coherencia |
| MongoDB + Atlas Vector Search | Dependencia externa adicional; pgvector ya cubre el caso |
| HNSW index siempre | IVFFlat es más simple y suficiente para < 1M registros |
| Embedding en columna JSONB | pgvector es más eficiente y tiene operadores dedicados |

## Diagrama

```
Documento raw (5000 tokens)
  │
  ▼
RecursiveCharacterTextSplitter
  ├── chunk[0]: tokens 0-512    → embedding[0] → INSERT INTO knowledge_chunks
  ├── chunk[1]: tokens 462-974  → embedding[1] → INSERT INTO knowledge_chunks
  ├── chunk[2]: tokens 924-1436 → embedding[2] → INSERT INTO knowledge_chunks
  └── ...

Búsqueda:
  "consulta del usuario"
    │
    ▼
  Embedder → vector(1536)
    │
    ▼
  SELECT *, embedding <=> $query AS distance
  FROM knowledge_chunks
  ORDER BY distance
  LIMIT 5

Hybrid:
  SELECT *,
    (1 - (embedding <=> $vector)) * 0.7 +
    ts_rank(tsv, plainto_tsquery('spanish', $text)) * 0.3
    AS hybrid_rank
  FROM knowledge_chunks
  WHERE tsv @@ plainto_tsquery('spanish', $text)
     OR embedding <=> $vector < 0.8
  ORDER BY hybrid_rank DESC
  LIMIT 5
```

## TDD plan

1. **Red**: Test: recursive chunker divide texto largo en chunks de tamaño correcto
2. **Green**: Implementar RecursiveCharacterTextSplitter
3. **Red**: Test: insertar documento → chunks creados con índice secuencial
4. **Green**: Implementar KnowledgeStore con CreateDocument + CreateChunks batch
5. **Red**: Test: semantic search con vector mock → resultados ordenados por distancia
6. **Green**: Implementar SemanticSearch con pgvector
7. **Red**: Test: hybrid search combina vector + keyword
8. **Green**: Implementar HybridSearch
9. **Sabotaje**: dropear índice IVFFlat → search funciona pero lento; recrear → performance normal

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| pgvector no instalado | Verificar extensión al iniciar; error claro |
| Embedding API caída | Cache local + retry con backoff |
| Dimensión incorrecta | Validar al crear índice; DROP y recreate si cambia |
| Chunk overlap excesivo | Configurable; default 50 tokens (~10%) |
