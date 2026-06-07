# Tasks: HU-03.7-cross-project-global-search

- [ ] **gs-001**: Migración `searchable_entities` materialized view
- [ ] **gs-002**: Cron refresh CONCURRENTLY cada 1-5min
- [ ] **gs-003**: Migración `saved_searches`
- [ ] **gs-004**: Cache `user_accessible_projects` con TTL 1min
- [ ] **gs-005**: Service `internal/service/search.go` con query híbrida
- [ ] **gs-006**: Handler GET /api/v1/search
- [ ] **gs-007**: Handlers CRUD saved_searches
- [ ] **gs-008**: Suggestions con `pg_trgm` similarity
- [ ] **test-001**: Híbrido ranking
- [ ] **test-002**: RBAC bob no ve project Y
- [ ] **test-003**: Filtros entity_type/project/date
- [ ] **test-004**: Performance 100k rows p99 <500ms
- [ ] **test-005**: Saved searches CRUD + run
- [ ] **test-006**: Suggestions empty case
- [ ] **docs-001**: `docs/search.md` con sintaxis + ejemplos
