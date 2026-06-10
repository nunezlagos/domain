# Proposal: issue-05.2-skill-registry-search

## Intención

Implementar el registro central de skills con capacidades de búsqueda dual: full-text search (tsvector) para coincidencia textual, y búsqueda semántica (cosine similarity via pgvector) para encontrar skills por significado. Modo híbrido que combina ambos scores.

## Scope

**Incluye:**
- Endpoint GET /api/skills (búsqueda textual + filtros)
- Endpoint POST /api/skills/search (búsqueda semántica e híbrida)
- Índice GIN sobre tsvector generado desde name + description
- Índice IVFFlat sobre embedding vector
- Modos: "fts", "semantic", "hybrid"
- Paginación con limit/offset
- Filtros combinables: type, project_id, tags (array overlap)
- Score de relevancia en modo semántico/híbrido

**Excluye:**
- Búsqueda por parámetros (solo por metadata del skill)
- Búsqueda en contenido del skill (solo name + description + tags)
- Re-ranking (fase posterior)

## Enfoque técnico

- Columna `search_vector` TSVECTOR generada desde `name || ' ' || COALESCE(description, '')`.
- Trigger BEFORE INSERT/UPDATE para mantener `search_vector` actualizado.
- Query FTS: `WHERE search_vector @@ plainto_tsquery('spanish', $1)`.
- Query semántica: `ORDER BY embedding <=> $1 LIMIT $2` (cosine distance).
- Modo híbrido: ejecutar ambas queries y combinar scores con pesos configurables.
- Endpoint POST /api/skills/search acepta JSON: `{ query, mode, top_k, threshold, fts_weight, semantic_weight, filters }`.

## Riesgos

- Performance en datasets grandes: índice IVFFlat requiere tuning de `lists` y `probes`. Mitigación: valor por defecto lists=100, probes=1, documentar para ajuste.
- Idioma en FTS: los skills pueden estar en español o inglés. Mitigación: configurar diccionario como `simple` para no depender del idioma.
- Embedding nulo: skills sin embedding se excluyen automáticamente de búsqueda semántica.

## Testing

- **Unitarios:** Generación de query FTS, combinación híbrida de scores, filtros combinados.
- **Integración:** Insertar skills, buscar por FTS, semántica e híbrido, verificar scores y orden.
- **E2E:** Búsqueda real con datos seed.
- **Sabotaje:** Skills sin embedding no aparecen en búsqueda semántica.
