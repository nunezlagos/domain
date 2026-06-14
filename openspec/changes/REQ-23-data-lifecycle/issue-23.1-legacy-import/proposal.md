# Proposal: issue-23.1-legacy-import

## Intención

Sistema de importers plug-and-play para migrar desde Markdown vault, JSON dump, Notion export y Obsidian vault. Jobs async con reporte detallado, idempotencia por hash, mapping configurable a entidades Domain.

## Scope

**Incluye:**
- Interfaz `Importer{Format() string; Parse(reader) (Items, error)}`
- 4 implementaciones iniciales: markdown-vault, json-dump, notion, obsidian
- Job tracking en tabla `import_jobs` con status/counts/errors
- Endpoint POST /imports + GET /imports/:job_id
- Procesamiento async via worker pool (similar a notifications)
- Storage temporal de upload en S3 con TTL 7 días

**No incluye:**
- Importers Roam/Logseq/Bear (futuro)
- Export inverso (issue-23.3 cubre export GDPR)

## Enfoque técnico

1. Upload a S3 temp bucket primero, luego procesar
2. Worker procesa job: parse → dedup por hash(content) → insert
3. Errores no abortan: collect y reportar al final
4. Soporte resumable parcial para imports grandes (>1000 items)

## Riesgos

- Parsing variado: tests con fixtures reales (Obsidian, Notion exports)
- Memoria: streaming en lugar de cargar todo
- Conflictos slug: estrategia documentada

## Testing

- Fixtures de cada formato + assert counts
- Idempotencia: 2do import skip todo
- Error en item N → resto procesa
- Performance: 1000 items en <60s
