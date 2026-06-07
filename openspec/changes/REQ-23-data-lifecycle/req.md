# REQ-23-data-lifecycle: Ciclo de vida de datos: import desde sistemas legacy, soft-delete con papelera/restore, export GDPR.

**Estado:** activo
**Creado:** 2026-06-06
**Fase:** F4

## Descripción

Gestión del ciclo de vida de los datos del usuario: importadores para migrar desde plataformas externas (Notion, Obsidian, Markdown, JSON dumps), soft-delete con papelera y restore por TTL, y export GDPR completo de datos del usuario en formato portable.

## Criterios de éxito

- Importadores plug-and-play para: Markdown vault, JSON dump, Notion export, Obsidian vault — mapeados a observations/knowledge_docs/prompts
- Soft-delete uniforme: campos `deleted_at`, vistas filtradas por defecto, papelera por entidad con TTL configurable (default 30d) y purge job
- Restore desde papelera con validación de no conflictos (slug/unique) y audit log del restore
- Export GDPR: endpoint que genera ZIP con todos los datos del usuario (JSON + adjuntos) en formato auto-documentado
- Tiempo objetivo: import 10MB <60s, export GDPR de cuenta promedio <5 min

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-23.1-legacy-import | proposed | Importadores Markdown vault, JSON dump, Notion export, Obsidian, mapping a entidades |
| HU-23.2-soft-delete-restore | proposed | Soft-delete uniforme con papelera, restore, TTL y purge job |
| HU-23.3-gdpr-export | proposed | Export GDPR ZIP con JSON + adjuntos, signed URL temporal |
