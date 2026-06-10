# Design: issue-03.1-observations-crud-fts

## Decisión arquitectónica

**Tabla única con generated column tsvector + GIN index.**

```
observations
├── id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── created_by      UUID NOT NULL REFERENCES users(id)
├── title           TEXT NOT NULL
├── content     TEXT NOT NULL
├── type        VARCHAR(50) NOT NULL        -- fix | decision | pattern | context | artifact | session
├── project_id  UUID NOT NULL REFERENCES projects(id)
├── scope       VARCHAR(20) NOT NULL DEFAULT 'project'  -- project | personal | global
├── tsv         TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', coalesce(title,'') || ' ' || coalesce(content,''))) STORED
├── created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Índices:**
- `observations_tsv_idx` GIN (tsv)
- `observations_type_idx` BTREE (type)
- `observations_project_idx` BTREE (project_id)
- `observations_scope_idx` BTREE (scope)

**Capa de acceso:**
- Repositorio con queries parametrizadas (evitar SQL injection)
- Search retorna `ObservationSearchResult` con `Rank float64` y `Headline string`
- Conflict detection query separada con threshold configurable

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| SQLite FTS5 (como engram) | No disponible en Postgres; no escala horizontalmente |
| Elasticsearch como backend de búsqueda | Overkill para el volumen; agrega dependencia externa |
| pg_trgm + LIKE | Menor precisión que tsvector para stemming y ranking |
| tsvector en columna regular (no generated) | Mayor propensión a errores de sincronización; generated column garantiza consistencia |
| Un índice GIN sobre múltiples columnas sin tsvector | No aprovecha stemming, ranking, ni stop words |

## Diagrama

```
┌──────────────────────────────────────────────┐
│                  Client                      │
│        (agent / CLI / HTTP API)              │
└──────────┬───────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────┐
│           MemoryService                      │
│      (lógica de negocio, conflict check)     │
└──────────┬───────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────┐
│        ObservationStore (interface)          │
│  Insert / GetByID / Update / Delete / Search │
└──────────┬───────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────┐
│        pgObservationStore (impl)             │
│  Queries SQL con sqlx, tsvector, GIN index   │
└──────────┬───────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────┐
│           PostgreSQL (tsvector + GIN)        │
└──────────────────────────────────────────────┘
```

## TDD plan

1. **Red**: Escribir test que inserta observación y busca por contenido
2. **Green**: Implementar migración + store.Insert + store.Search mínimo
3. **Refactor**: Agregar generated column, índices, filtros
4. **Red**: Escribir test de conflict detection
5. **Green**: Implementar conflict query con ts_rank threshold
6. **Refactor**: Parametrizar threshold, extraer a función reusable
7. **Sabotaje**: Eliminar índice → test search debe fallar; re-crear índice → test pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| generated column no soportada en versión antigua de PG | Requerir PG >= 12 (generated columns desde 12) |
| tsvector no indexado en bulk insert | Batch insert con GIN maintenance |
| Ranking no preciso para ciertos idiomas | Usar diccionario spanish, fallback a simple si es necesario |
