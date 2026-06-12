# Tasks: issue-04.7-interactive-hu-builder

## Schema

- [x] **hb-001**: Migration issue_drafts + hu_draft_steps_log
- [x] **hb-002**: Trigger updated_at + cleanup cron stub

## Service

- [x] **hb-010**: Package internal/sdd/wizard skeleton (Service, types)
- [x] **hb-011**: Step interface + Option struct
- [x] **hb-012**: flowsByMode registry
- [x] **hb-013**: flow_feature (8 steps)
- [x] **hb-014**: flow_bugfix (6 steps)
- [x] **hb-015**: flow_refactor (5 steps)
- [x] **hb-016**: flow_doc (3 steps)
- [x] **hb-017**: flow_rfc (7 steps)
- [x] **hb-018**: validators (audience exists, req exists, slug format, path safe)
- [x] **hb-019**: Start/Answer/Preview/Commit/Abandon methods

## Templates

- [x] **hb-020**: go:embed templates/
- [x] **hb-021**: hu.md.tmpl
- [x] **hb-022**: proposal.md.tmpl
- [x] **hb-023**: design.md.tmpl
- [x] **hb-024**: tasks.md.tmpl
- [x] **hb-025**: state.yaml.tmpl
- [x] **hb-026**: rfc.md.tmpl
- [x] **hb-027**: renderer + inferpath helpers

## MCP tools

- [x] **hb-030**: domain_hu_create_start
- [x] **hb-031**: domain_hu_create_answer
- [x] **hb-032**: domain_hu_create_preview
- [x] **hb-033**: domain_hu_create_commit
- [x] **hb-034**: domain_hu_create_abandon
- [x] **hb-035**: domain_issue_drafts_list
- [x] **hb-036**: Tool descriptions + JSON schemas claras para LLM

## CLI

- [x] **hb-040**: `domain hu create [--mode]` con prompts terminal
- [x] **hb-041**: `domain hu draft list`
- [x] **hb-042**: `domain hu draft show :id`

## HTTP API (futuro, soporte Web UI)

- [x] **hb-050**: POST /api/v1/hu-drafts (start)
- [x] **hb-051**: PATCH /api/v1/hu-drafts/:id/answer
- [x] **hb-052**: GET /api/v1/hu-drafts/:id/preview
- [x] **hb-053**: POST /api/v1/hu-drafts/:id/commit

## Cron

- [x] **hb-060**: Cron purge drafts status=in_progress AND expires_at < now (cada 1h)
- [x] **hb-061**: Cron purge drafts >7d en cualquier status

## Tests

- [x] **hb-070**: Service.Start crea draft + retorna primer step
- [x] **hb-071**: Answer válida avanza step
- [x] **hb-072**: Answer inválida (audience slug fake) retorna error + opciones again
- [x] **hb-073**: Flujo full feature 8 respuestas → status=finished + preview
- [x] **hb-074**: Preview genera 5 archivos
- [x] **hb-075**: Commit escribe filesystem + audit log
- [x] **hb-076**: Expired draft → ErrExpired en Commit
- [x] **hb-077**: MCP tools wrap correcto
- [x] **hb-078**: Path inference: next HU number correcto desde existing
- [x] **hb-079**: Slug-ify reemplaza espacios, normaliza, max 50 chars
- [x] **sabotaje-001**: Concurrent answer mismo draft → optimistic lock previene last-write-wins
- [x] **sabotaje-002**: Commit a path fuera de openspec/changes/ → reject (path traversal)

## Docs

- [x] **hb-080**: docs/sdd/interactive-builder.md con flow diagrams + ejemplos
- [x] **hb-081**: README sample MCP session usando tool
