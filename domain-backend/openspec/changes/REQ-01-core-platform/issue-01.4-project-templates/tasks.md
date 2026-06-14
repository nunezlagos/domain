# Tasks: issue-01.4-project-templates

> Nota de schema: no existe tabla project_skills — skills/agents/flows son
> org-scoped, no project-scoped. Los arrays default_skills/agents/flows del
> template son una lista de recomendación (consumible por el agente MCP),
> NO una asignación automática: la auto-asignación queda N/A hasta que
> exista asociación per-project en el schema.

## Backend

- [x] Migración → 000021 project_templates (org nullable, is_public, defaults TEXT[], seed_managed/is_user_modified) + FK projects.template_id
- [x] Service → internal/service/projecttemplate (Create/Get/GetBySlug/ListByOrg/Delete; Get acepta org propia O is_public)
- [x] CLI cobra → N/A por diseño (gestión vía HTTP /api/v1/project-templates; el CLI usa el API client)
- [x] Seed built-in → ProjectTemplatesSeeder: default (is_default), go-backend, python-data, frontend-web; org NULL + is_public; upsert manual (UNIQUE no aplica con org NULL) respetando is_user_modified — 2026-06-11
- [x] ProjectService.Create con template_id → merge settings (template base + request override); defaults arrays son recomendación (ver nota)

## Tests

- [x] Create template settings válidos → suite projecttemplate
- [x] Slug duplicado → ErrSlugTaken (UNIQUE org+slug)
- [x] Proyecto desde template hereda settings → merge verificado en projectsvc.Create (skills N/A ver nota)
- [x] Update template no afecta proyectos existentes → settings se COPIAN al crear (no referencia viva), por construcción
- [x] Settings inválidos → validación slug/shape en service
- [x] Seed al boot → TestProjectTemplatesSeeder_BuiltinsAndUserModified (4 creados, 1 default, públicos) — 2026-06-11
- [x] Sabotaje user-modified → re-seed preserva edición del usuario (skipped=1) — 2026-06-11
- [x] Sabotaje delete template → FK ON DELETE SET NULL: proyectos intactos con template_id NULL

## Cierre

- [x] Verificación → cubierta por tests integration (mismo código)
- [x] Suite verde → 2026-06-11
