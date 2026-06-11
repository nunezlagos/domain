# Tasks: issue-03.7-cross-project-global-search

> Decisión de producto 2026-06-10 (MCP-first, sin Web UI): la búsqueda
> global se consume vía `domain_search_global` (MCP) y GET /api/v1/search.
> Implementación: FTS per-entity + merge por score (4 entities), sin
> materialized view — a la escala actual cada query con índice GIN propio
> rinde sin MV. saved_searches y suggestions son features de UI → DIFERIDOS.

- [ ] **gs-001**: MV `searchable_entities` → DIFERIDO (FTS per-entity con GIN por tabla; MV se justifica recién con volumen que degrade el merge)
- [ ] **gs-002**: Cron refresh MV → DIFERIDO con gs-001
- [ ] **gs-003**: Migración `saved_searches` → DIFERIDO (feature de Web UI; el agente MCP re-emite queries)
- [ ] **gs-004**: Cache `user_accessible_projects` → DIFERIDO (scoping actual por organization_id; RBAC granular por project llega con issue-02.8)
- [x] **gs-005**: Service `internal/service/search` → Search unificado sobre observations + prompts + sessions + knowledge_docs (ts_rank, merge por score DESC, Filter EntityTypes/ProjectIDs/Tags/DateFrom-To)
- [x] **gs-006**: Handler GET /api/v1/search → ?q&limit&entity_type&project_slug&tags&date_from&date_to
- [x] **gs-006b**: MCP tool `domain_search_global(query, limit?, entity_types[], tags[])` — el consumidor principal del producto
- [ ] **gs-007**: Handlers CRUD saved_searches → diferido con gs-003
- [ ] **gs-008**: Suggestions pg_trgm → DIFERIDO (CLI ya sugiere typos vía Levenshtein issue-14.3; suggestions de contenido es UX de UI)
- [x] **test-001**: Ranking unificado → search 4 entity types con misma keyword, merge por score
- [x] **test-002**: Aislamiento → sabotaje cross-org devuelve 0 resultados (scoping organization_id; RBAC por project diferido con gs-004)
- [x] **test-003**: Filtros entity_type/tags/date → Filter EntityTypes prompt-only, Tags AND, DateFrom futuro → 0
- [ ] **test-004**: Performance 100k p99 <500ms → DIFERIDO (benchmark manual/CI weekly; sin dataset de 100k aún)
- [ ] **test-005**: Saved searches → diferido con gs-003
- [ ] **test-006**: Suggestions → diferido con gs-008
- [ ] **docs-001**: `docs/search.md` → diferido; sintaxis documentada en el tool description de domain_search_global + handler
