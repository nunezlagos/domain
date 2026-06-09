# REQ-04-opsx-sdd: Integración del sistema SDD: requerimientos (REQs), historias de usuario (HUs), specs, diseños, tareas, verificación, trazabilidad.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F4

## Descripción

Integración del sistema SDD: requerimientos (REQs), historias de usuario (HUs), specs, diseños, tareas, verificación, trazabilidad.

## Criterios de éxito

- REQs, HUs y specs almacenados con trazabilidad completa
- Adjuntos (imágenes, diagramas) almacenados en S3 con signed URLs
- Cleanup automático de adjuntos huérfanos
- Wizard interactivo MCP/CLI para crear HUs/REQs/RFCs con preguntas dirigidas (5 modos: feature/bug-fix/refactor/doc/rfc) que clarifican persona, REQ padre, path, goals, pains, metrics antes de generar la spec
- Pipeline única de intake desde fuentes heterogéneas (agente IA inicialmente; email/webhook/sheets/slack como adapters futuros) con clasificación IA, deduplicación semántica, generación de Gherkin draft y review humano obligatorio
- Sincronización Domain ↔ proveedor externo (Jira Cloud en MVP) con detección de drift cuando se edita en el provider y resolución manual asistida; Domain es source of truth
- Audit immutable + state machine declarativa + métricas de lifecycle (lead_time/cycle_time/throughput) + stuck detector cron

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-04.1-requirements-crud | proposed | CRUD de REQs con slug único, descripción, estado, prioridad |
| HU-04.2-user-stories-gherkin | proposed | Creación y versionado de HUs con Gherkin scenarios |
| HU-04.3-specs-designs | proposed | Specs, ADRs, alternativas descartadas, diagramas |
| HU-04.4-tasks-verification | proposed | Tasks, tests, sabotaje, cierre, checklist |
| HU-04.5-traceability | proposed | Trazabilidad REQ→HU→tasks, reportes de cobertura |
| HU-04.6-s3-storage | proposed | S3 integration para adjuntos de opsx, knowledge docs y assets |
| HU-04.7-interactive-hu-builder | proposed | Wizard MCP/CLI con state en BD para generar HUs/REQs/RFCs con preguntas dirigidas; 5 modos; valida persona/REQ contra catálogo |
| HU-04.8-intake-pipeline | proposed | Pipeline ingestión multi-source con clasificación IA, dedupe semántico, generación Gherkin draft, human review y commit transaccional |
| HU-04.9-external-provider-sync | proposed | Sync Domain ↔ Jira Cloud (driver pattern para más providers) con push REQ/HU/attachments 1..N, pull webhook, drift detection y conflict resolution |
| HU-04.10-lifecycle-state-tracking | proposed | State machine declarativa, audit immutable cross-entity, optimistic locking, stuck detector cron, métricas lifecycle, compensation pattern |
