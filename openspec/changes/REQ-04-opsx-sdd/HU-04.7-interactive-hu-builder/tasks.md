# Tasks: HU-04.7-interactive-hu-builder

## Schema

- [ ] **hb-001**: Migration hu_drafts + hu_draft_steps_log
- [ ] **hb-002**: Trigger updated_at + cleanup cron stub

## Service

- [ ] **hb-010**: Package internal/sdd/wizard skeleton (Service, types)
- [ ] **hb-011**: Step interface + Option struct
- [ ] **hb-012**: flowsByMode registry
- [ ] **hb-013**: flow_feature (8 steps)
- [ ] **hb-014**: flow_bugfix (6 steps)
- [ ] **hb-015**: flow_refactor (5 steps)
- [ ] **hb-016**: flow_doc (3 steps)
- [ ] **hb-017**: flow_rfc (7 steps)
- [ ] **hb-018**: validators (audience exists, req exists, slug format, path safe)
- [ ] **hb-019**: Start/Answer/Preview/Commit/Abandon methods

## Templates

- [ ] **hb-020**: go:embed templates/
- [ ] **hb-021**: hu.md.tmpl
- [ ] **hb-022**: proposal.md.tmpl
- [ ] **hb-023**: design.md.tmpl
- [ ] **hb-024**: tasks.md.tmpl
- [ ] **hb-025**: state.yaml.tmpl
- [ ] **hb-026**: rfc.md.tmpl
- [ ] **hb-027**: renderer + inferpath helpers

## MCP tools

- [ ] **hb-030**: domain_hu_create_start
- [ ] **hb-031**: domain_hu_create_answer
- [ ] **hb-032**: domain_hu_create_preview
- [ ] **hb-033**: domain_hu_create_commit
- [ ] **hb-034**: domain_hu_create_abandon
- [ ] **hb-035**: domain_hu_drafts_list
- [ ] **hb-036**: Tool descriptions + JSON schemas claras para LLM

## CLI

- [ ] **hb-040**: `domain hu create [--mode]` con prompts terminal
- [ ] **hb-041**: `domain hu draft list`
- [ ] **hb-042**: `domain hu draft show :id`

## HTTP API (futuro, soporte Web UI)

- [ ] **hb-050**: POST /api/v1/hu-drafts (start)
- [ ] **hb-051**: PATCH /api/v1/hu-drafts/:id/answer
- [ ] **hb-052**: GET /api/v1/hu-drafts/:id/preview
- [ ] **hb-053**: POST /api/v1/hu-drafts/:id/commit

## Cron

- [ ] **hb-060**: Cron purge drafts status=in_progress AND expires_at < now (cada 1h)
- [ ] **hb-061**: Cron purge drafts >7d en cualquier status

## Tests

- [ ] **hb-070**: Service.Start crea draft + retorna primer step
- [ ] **hb-071**: Answer válida avanza step
- [ ] **hb-072**: Answer inválida (audience slug fake) retorna error + opciones again
- [ ] **hb-073**: Flujo full feature 8 respuestas → status=finished + preview
- [ ] **hb-074**: Preview genera 5 archivos
- [ ] **hb-075**: Commit escribe filesystem + audit log
- [ ] **hb-076**: Expired draft → ErrExpired en Commit
- [ ] **hb-077**: MCP tools wrap correcto
- [ ] **hb-078**: Path inference: next HU number correcto desde existing
- [ ] **hb-079**: Slug-ify reemplaza espacios, normaliza, max 50 chars
- [ ] **sabotaje-001**: Concurrent answer mismo draft → optimistic lock previene last-write-wins
- [ ] **sabotaje-002**: Commit a path fuera de openspec/changes/ → reject (path traversal)

## Docs

- [ ] **hb-080**: docs/sdd/interactive-builder.md con flow diagrams + ejemplos
- [ ] **hb-081**: README sample MCP session usando tool
