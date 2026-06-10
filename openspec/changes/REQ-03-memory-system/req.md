# REQ-03-memory-system: Sistema de memoria: observaciones con tsvector FTS, sesiones, prompts, documentos de conocimiento (RAG-ready), deduplicación, timeline, contexto.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F2

## Descripción

Sistema de memoria: observaciones con tsvector FTS, sesiones, prompts, documentos de conocimiento (RAG-ready), deduplicación, timeline, contexto.

## Criterios de éxito

- CRUD completo de observaciones con FTS (tsvector) y filtros por proyecto/usuario/tags/tipo
- Lifecycle de sesiones con start/end/resume, agrupación de observaciones y resumen automático al cerrar
- Almacenamiento de prompts con versionado, variables tipadas y referencia a su última ejecución
- Knowledge docs (markdown/texto) con chunking, embeddings y búsqueda híbrida (FTS + vectorial)
- Timeline cronológico/contextual que mezcla observaciones, sesiones, prompts y runs de un proyecto
- Deduplicación semántica (umbral configurable por embedding similarity) y redacción PII configurable
- Búsqueda global híbrida (tsvector + vector) cruzando proyectos del usuario, con saved searches y RBAC enforcement

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-03.1-observations-crud-fts | proposed | CRUD de observaciones con tsvector FTS, filtros por proyecto/tags/tipo, paginación |
| issue-03.2-sessions-lifecycle | proposed | Sesiones con start/end/resume, agrupación de observaciones, summary al cerrar |
| issue-03.3-prompts-storage | proposed | Almacenamiento de prompts con versionado, variables tipadas y referencia a runs |
| issue-03.4-knowledge-documents | proposed | Knowledge docs con chunking, embeddings, búsqueda híbrida FTS + vectorial |
| issue-03.5-context-timeline | proposed | Timeline cronológico/contextual que mezcla observaciones/sesiones/prompts/runs |
| issue-03.6-dedup-privacy | proposed | Deduplicación semántica por similitud de embeddings + redacción PII configurable |
| issue-03.7-cross-project-global-search | proposed | Búsqueda global híbrida FTS+vector cruzando proyectos/orgs, saved searches, RBAC scoped |
