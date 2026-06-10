# Tasks: issue-01.9-personas-catalog

## Schema + Data

- [ ] **pc-001**: Migration personas + hu_personas cross-ref
- [ ] **pc-002**: Seeds YAML para 10 personas built-in
- [ ] **pc-003**: Integration issue-01.7 seeders
- [ ] **pc-004**: trigger cache_invalidate (issue-26.7)

## Service

- [ ] **pc-010**: Service CRUD personas
- [ ] **pc-011**: Markdown→structured parser (similar issue-01.8)
- [ ] **pc-012**: Markdown renderer
- [ ] **pc-013**: Cross-ref builder (parse hu.md headers → populate hu_personas)

## MCP

- [ ] **pc-020**: Tool `domain_persona_get`
- [ ] **pc-021**: Tool `domain_persona_list`
- [ ] **pc-022**: Tool `domain_hus_for_persona`
- [ ] **pc-023**: Cache integration issue-12.6

## HTTP

- [ ] **pc-030**: Handler GET/POST/PATCH/DELETE /admin/personas
- [ ] **pc-031**: Handler GET /api/v1/personas (public list, RBAC member)
- [ ] **pc-032**: Handler GET /api/v1/personas/:slug/hus

## CLI

- [ ] **pc-040**: `domain personas list`
- [ ] **pc-041**: `domain personas get :slug`
- [ ] **pc-042**: `domain personas export-md --to`
- [ ] **pc-043**: `domain personas import-md --from`
- [ ] **pc-044**: `domain personas reindex-hu-cross-refs`

## Linter

- [ ] **pc-050**: Parser hu.md detect `**Persona:** xxx`
- [ ] **pc-051**: Validator slug exists
- [ ] **pc-052**: `.personas-baseline.json` skip legacy
- [ ] **pc-053**: CI integration

## Retrofit (issue-26 task separada)

- [ ] **pc-060**: Script Go que infiere persona desde keywords en cada hu.md
- [ ] **pc-061**: Aplicar a 148 HUs
- [ ] **pc-062**: Spot-check humano 10% muestras
- [ ] **pc-063**: PR retrofit con merge único

## Tests

- [ ] **pc-070**: CRUD + audit
- [ ] **pc-071**: Seed 10 personas
- [ ] **pc-072**: MCP tools (con cache hit/miss)
- [ ] **pc-073**: Export md round-trip
- [ ] **pc-074**: Import md parser
- [ ] **pc-075**: Linter sin field fail
- [ ] **pc-076**: Linter slug inexistente fail
- [ ] **pc-077**: Cross-ref table populated correctly tras parse
- [ ] **pc-078**: Retrofit script infiere correctamente sample HUs

## Docs

- [ ] **pc-080**: `docs/personas.md` generado desde BD (10 personas full detail)
- [ ] **pc-081**: `docs/architecture/personas.md` explicando concepto + workflow
