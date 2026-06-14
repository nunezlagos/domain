# Proposal: issue-06.5-embedding-pgvector

## Intención

Implementar providers de embeddings para OpenAI y Anthropic, e integrarlos con pgvector para almacenamiento y búsqueda semántica. Esto es la base para búsqueda semántica de skills (issue-05.2), recomendación de skills (issue-05.4), y eventualmente búsqueda semántica de observaciones.

## Scope

**Incluye:**
- Interface `Embedder`: Embed, EmbedBatch, Name, Dimensions
- `OpenAIEmbedder`: usa text-embedding-3-small (1536 dimensiones)
- `AnthropicEmbedder`: usa modelo de embeddings de Anthropic
- Migración SQL: CREATE EXTENSION vector
- Columnas `vector(1536)` en tablas skills, observations (existente)
- Índices IVFFlat (default) y HNSW (opt-in)
- Helper queries para cosine similarity
- Embedding de textos combinados (name + description) para skills

**Excluye:**
- Embedding de documentos largos (chunking)
- Cache de embeddings (issue-05.4 tiene cache de contexto)
- Re-ranking

## Enfoque técnico

- Embedder interface separada de Provider interface (son responsabilidades distintas).
- OpenAI embedder: POST https://api.openai.com/v1/embeddings con model "text-embedding-3-small".
- Anthropic embedder: usa el endpoint de embeddings de Anthropic.
- Ambos retornan []float32 normalizados.
- La migración SQL ejecuta `CREATE EXTENSION IF NOT EXISTS vector`.
- Las columnas embedding se agregan vía migración a skills y observations.
- Índices se crean con `USING ivfflat` por defecto, `USING hnsw` como alternativa.

## Riesgos

- **pgvector no instalado:** La migración falla. Documentar como prerequisito.
- **Dimensión incorrecta:** Si cambiamos de modelo de embedding, la columna no coincide. Mitigación: dimension configurable, migrations para cambio de dimensión.
- **Costo de embedding:** Textos grandes cuestan más tokens. Mitigación: truncar a 512 tokens antes de embedder.

## Testing

- **Unitarios:** Embedder interface, mock HTTP, batch embedding, texto vacío.
- **Integración:** Migración pgvector, inserción y búsqueda en DB real.
- **Sabotaje:** API key inválida → error graceful.
