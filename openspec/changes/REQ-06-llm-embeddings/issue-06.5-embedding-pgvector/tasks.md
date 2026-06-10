# Tasks: issue-06.5-embedding-pgvector

## Backend

- [ ] Definir interface Embedder con Name, Dimensions, Embed, EmbedBatch
- [ ] Implementar OpenAIEmbedder (text-embedding-3-small)
- [ ] Implementar AnthropicEmbedder
- [ ] Crear migración SQL: CREATE EXTENSION vector
- [ ] Crear migración: agregar columna embedding a skills
- [ ] Crear migración: agregar columna embedding a observations
- [ ] Crear índices IVFFlat (y opcional HNSW)
- [ ] Implementar helper queries para cosine similarity
- [ ] Implementar truncado a 512 tokens antes de embedder
- [ ] Integrar generación de embedding en creación/actualización de skills

## Frontend

- [ ] N/A

## Tests

- [ ] Test unitario: OpenAI embedder con mock HTTP
- [ ] Test unitario: Anthropic embedder con mock HTTP
- [ ] Test unitario: EmbedBatch produce vectores correctos
- [ ] Test unitario: texto vacío manejado
- [ ] Test unitario: truncado a 512 tokens
- [ ] Test integración: migración pgvector en DB test
- [ ] Test integración: insertar embedding y buscar por similitud
- [ ] Sabotaje: API key inválida → error graceful

## Cierre

- [ ] Verificar migración en DB local
- [ ] Suite verde
