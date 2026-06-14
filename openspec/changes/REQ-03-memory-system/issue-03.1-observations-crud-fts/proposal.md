# Proposal: issue-03.1-observations-crud-fts

## Intención

Implementar el CRUD completo de observaciones con full-text search nativo de Postgres usando tsvector y GIN index. Reemplazar el approach de SQLite FTS5 del engram original por tsvector/tsquery, manteniendo la misma semántica de búsqueda pero aprovechando las capacidades de Postgres.

## Scope

**Incluye:**
- Tabla `observations` con migración versionada (tipo: `sqlx` o `golang-migrate`)
- Columnas: `id` (UUID PK), `created_by` (UUID FK → users(id)), `title`, `content`, `type`, `project_id` (UUID FK → projects(id)), `scope`, `tsvector` (generated column), `created_at`, `updated_at`
- Índice GIN sobre `tsvector`
- CRUD: `Insert`, `GetByID`, `Update`, `Delete`, `Search`
- Search con `plainto_tsquery` + `ts_rank`, más `ts_headline` para snippets
- Filtros combinables: `type`, `project`, `scope`, `limit`
- Conflict detection: antes de insertar, buscar candidatos con `ts_rank > 0.7` y devolverlos
- Tests de integración con base de datos real (testcontainers o similar)

**Excluye:**
- Cliente HTTP/API (se hará en REQ-13)
- Interfaz de usuario (se hará en REQ-16)
- Deduplicación por hash (es issue-03.6)

## Enfoque técnico

1. **Migración**: crear tabla con generated column `tsvector` usando `to_tsvector('spanish', coalesce(title,'') || ' ' || coalesce(content,''))`
2. **GIN index**: `CREATE INDEX observations_tsv_idx ON observations USING GIN(tsv);`
3. **Search query**: `SELECT *, ts_rank(tsv, plainto_tsquery('spanish', $1)) AS rank, ts_headline('spanish', content, plainto_tsquery('spanish', $1)) AS headline FROM observations WHERE tsv @@ plainto_tsquery('spanish', $1) ORDER BY rank DESC LIMIT $2`
4. **Conflict detection**: misma query de search con threshold de rank > 0.7, devolver top 3 candidatos
5. **Capa Go**: `internal/store/pg/observation.go` con interfaz `ObservationStore` y structs tipados

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Performance de GIN con alta concurrencia de writes | Medio | Usar `gin__pending_list_limit` y benchmark con pgbench |
| Stop words en español afectan búsqueda | Medio | Configurar diccionario de spanish stemmer, permitir override a english |
| generated column no actualiza en UPDATE si no cambian title/content | Bajo | Usar trigger BEFORE UPDATE si es necesario, o recalcular en app |

## Testing

- **Unitarios**: mock de store, probar lógica de negocio
- **Integración**: pgtest con container Postgres, probar inserts, search, conflict detection
- **Regression**: probar que tsvector se genera correctamente con caracteres especiales, unicode, empty strings
- **Sabotaje**: romper el índice GIN → search debe caer con error claro
