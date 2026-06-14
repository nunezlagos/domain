# Tasks: issue-06.5-embedding-pgvector

## Backend

- [x] Definir interface Embedder con Name, Dimensions, Embed, EmbedBatch
- [x] Implementar OpenAIEmbedder (text-embedding-3-small)
- [x] Implementar AnthropicEmbedder
- [x] Crear migración SQL: CREATE EXTENSION vector
- [x] Crear migración: agregar columna embedding a skills
- [x] Crear migración: agregar columna embedding a observations
- [x] Crear índices IVFFlat (y opcional HNSW)
- [x] Implementar helper queries para cosine similarity
- [x] Implementar truncado a 512 tokens antes de embedder
- [x] Integrar generación de embedding en creación/actualización de skills

## Frontend

- [x] N/A

## Tests

- [x] Test unitario: OpenAI embedder con mock HTTP
- [x] Test unitario: Anthropic embedder con mock HTTP
- [x] Test unitario: EmbedBatch produce vectores correctos
- [x] Test unitario: texto vacío manejado
- [x] Test unitario: truncado a 512 tokens
- [x] Test integración: migración pgvector en DB test
- [x] Test integración: insertar embedding y buscar por similitud
- [x] Sabotaje: API key inválida → error graceful

## Cierre

- [x] Verificar migración en DB local
- [x] Suite verde
