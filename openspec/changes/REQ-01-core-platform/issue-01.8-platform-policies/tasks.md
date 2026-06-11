# Tasks: issue-01.8-platform-policies

> Decisión de producto 2026-06-10 (MCP-first, sin Web UI): el valor core es
> que un agente conectado por MCP lea las rules desde BD (`domain_policy_get`).
> Parser md→structured, renderer, FTS, cache LRU y rollback HTTP quedan
> DIFERIDOS hasta que haya demanda real; `body_structured` se acepta como
> JSONB opaco vía API.

- [x] **pp-001**: Migration platform_policies + versions → 000045 (índices kind/slug; FTS index diferido con pp-005)
- [x] **pp-002**: Service CRUD + versioning → Create/GetBySlug/List/Update (archiva en platform_policy_versions, atómico)/Delete soft. Rollback → diferido con pp-008
- [ ] **pp-003**: Markdown → body_structured parser → DIFERIDO (MCP-first: body_md es la fuente que consume el agente)
- [ ] **pp-004**: body_structured → markdown renderer → DIFERIDO (ídem; export-md usa body_md directo)
- [ ] **pp-005**: FTS search service → DIFERIDO (domain_policy_list + GetBySlug cubren descubrimiento con <30 policies)
- [ ] **pp-006**: In-proc cache LRU 5min → DIFERIDO (se evalúa junto a issue-12.6 tool configs)
- [x] **pp-007**: Endpoint CRUD `/api/v1/platform/policies` → POST/GET/GET{slug}/PATCH/DELETE
- [ ] **pp-008**: Endpoint versions list + rollback → DIFERIDO (history ya se persiste; falta GET versions + activate)
- [x] **pp-009**: MCP tool `domain_policy_get` → policy_tools.go ({slug,name,kind,version,body_md}) — 2026-06-11
- [x] **pp-010**: MCP tool descubrimiento → `domain_policy_list` (filtro kind) en lugar de search FTS — 2026-06-11; `domain_policies_search` diferido con pp-005
- [x] **pp-011**: CLI `domain policies import-md --dir` → front matter + kind.slug.md + --dry-run
- [x] **pp-012**: CLI `domain policies export-md --dir` → escribe kind.slug.md con front matter
- [x] **pp-013**: Integration issue-01.7 seeders → platform_policies_seeder.go (Order 30, no DevOnly)
- [ ] **pp-014**: Linters issue-25.13/13.9 leen body_structured → DIFERIDO (linters actuales son AST-based y autocontenidos)
- [x] **test-001**: CRUD + audit → service_test.go (validKinds, slug/kind inválidos, sentinels)
- [x] **test-002**: Versionado histórico → Update archiva versión (cubierto por integración API)
- [ ] **test-003**: Rollback → diferido con pp-008
- [x] **test-004**: Import .md → cubierto por CLI import-md (dry-run + front matter)
- [x] **test-005**: Export round-trip → export-md regenera front matter consistente con import
- [ ] **test-006**: FTS search → diferido con pp-005
- [ ] **test-007**: Parser fixtures → diferido con pp-003
- [x] **test-008**: MCP get + list → TestMCP_PolicyGetAndList (get devuelve body, list total, slug inexistente → error) — 2026-06-11
- [ ] **test-009**: Cache hit/miss → diferido con pp-006
- [x] **test-010**: is_user_modified preserva → migration 000082 + guard CASE WHEN en seeder + Update marca TRUE → TestPlatformPoliciesSeeder_PreservesUserModified (sabotaje incluido) — 2026-06-11
- [ ] **docs-001**: `docs/policies.md` → diferido; el workflow vive en help del CLI + este tasks.md
