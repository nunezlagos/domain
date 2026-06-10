# Proposal: issue-03.7-cross-project-global-search

## Intención

Endpoint `/search` global híbrido (FTS tsvector + vector cosine) sobre todas las entidades textuales del user (observations, knowledge_docs, prompts, sessions metadata), respetando RBAC. Sin filtro obligatorio de project.

## Scope

**Incluye:**
- Endpoint GET /api/v1/search con query + filtros opcionales
- Búsqueda híbrida con score combinado configurable
- RBAC scoping eficiente vía vista materializada o índice partial
- Saved searches (tabla `saved_searches` con CRUD)
- Performance p99 <500ms en 100k entidades
- Suggestions cuando no hay matches

**No incluye:**
- Search across orgs en las que NO soy miembro
- Full-text search en campos no-textuales (embeddings de imágenes, etc.)
- Realtime indexing differential (eventual consistency aceptable)

## Enfoque técnico

1. View materializada `searchable_entities` que union-all desde tablas con columnas comunes (id, entity_type, project_id, content, content_tsv, embedding, created_at, tags)
2. Refresh incremental con triggers o CDC simple
3. Query híbrida con CTE: FTS + vector top-K → merge by id → re-rank por score híbrido
4. RBAC: precompute lista de project_ids accesibles por user en cache (TTL 1min)

## Riesgos

- Costo de view materializada con dataset grande → particionar por org_id
- RBAC filtering inefficient si se hace post-query → precompute project_ids accesibles
- Ranking sesgado a un lado → exponer pesos en query string para tuning

## Testing

- Híbrido devuelve resultados de ambas dimensiones
- RBAC: bob no ve datos de project Y
- Performance benchmark 100k rows
- Saved searches CRUD + run
- Suggestions cuando empty
