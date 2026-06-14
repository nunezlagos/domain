# Design: issue-06.5-embedding-pgvector

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| Embedder interface | Separada de Provider | Unificada | Responsabilidades distintas |
| Dimensión default | 1536 (text-embedding-3-small) | Configurable | Modelo más usado de OpenAI |
| Index default | IVFFlat | HNSW | Menos memoria, tuning simple |
| Text truncation | 512 tokens antes de embed | Sin límite | Control de costos |

## Alternativas descartadas

- **Interface unificada Provider+Embedder:** Embedding no necesita streaming ni los mismos opts. Separación es más limpia.
- **HNSW por defecto:** Mejor recall pero más memoria. IVFFlat es default con recomendación de HNSW para producción grande.

## Diagrama

```
internal/llm/embedding/
├── embedder.go           ← Embedder interface
├── openai.go             ← OpenAIEmbedder
├── anthropic.go          ← AnthropicEmbedder
└── pgvector.go           ← Helper queries

internal/database/migrations/
└── 002_embedding.up.sql
    ├── CREATE EXTENSION vector
    ├── ALTER TABLE skills ADD COLUMN embedding vector(1536)
    ├── ALTER TABLE observations ADD COLUMN embedding vector(1536)
    ├── CREATE INDEX ivfflat_skills_embedding ON skills USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)
    └── CREATE INDEX ivfflat_obs_embedding ON observations USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)
```

### Interface

```go
type Embedder interface {
    Name() string
    Dimensions() int
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}
```

### Helper queries

```go
// Cosine similarity search
func CosineSimilarityQuery(embedding []float32, limit int) string {
    return fmt.Sprintf(`
        SELECT id, 1 - (embedding <=> '%s'::vector) AS similarity
        FROM skills
        WHERE embedding IS NOT NULL
        ORDER BY similarity DESC
        LIMIT %d
    `, pgvector.NewVector(embedding), limit)
}
```

## TDD plan

1. **TestOpenAIEmbed:** Mock HTTP → vector 1536
2. **TestOpenAIEmbedBatch:** 3 textos → 3 vectores
3. **TestAnthropicEmbed:** Mock HTTP → vector correcto
4. **TestEmbedderDimensions:** Dimensions() coincide con output
5. **TestTextoVacio:** Embed("") → error o vector zeros
6. **TestPgvectorExtension:** Migración crea extension
7. **TestPgvectorInsertAndSearch:** Insertar → buscar por similitud → orden correcto
8. **TestPgvectorIVFFlatIndex:** Índice mejora performance (EXPLAIN ANALYZE)
9. **TestEmbeddingTruncation:** Texto > 512 tokens se trunca
10. **TestSabotaje:** API key inválida → error graceful

## Riesgos y mitigación

- **pgvector no disponible:** Error en migración. Documentar como prerequisito en README.
- **Costo de embedding:** Cache de embeddings de contenido estático. Truncar textos largos.
- **Dimension mismatch:** Si cambiamos modelo, necesitamos re-embedder todos los registros. Migración con backfill.
