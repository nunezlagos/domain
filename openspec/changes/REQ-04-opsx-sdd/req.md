# REQ-04-opsx-sdd: Integración del sistema SDD: requerimientos (REQs), historias de usuario (HUs), specs, diseños, tareas, verificación, trazabilidad.

**Estado:** activo
**Creado:** 2026-06-07

## Descripción

Integración del sistema SDD: requerimientos (REQs), historias de usuario (HUs), specs, diseños, tareas, verificación, trazabilidad.

## Criterios de éxito

- REQs, HUs y specs almacenados con trazabilidad completa
- Adjuntos (imágenes, diagramas) almacenados en S3 con signed URLs
- Cleanup automático de adjuntos huérfanos

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-04.1-requirements-crud | proposed | CRUD de REQs con slug único, descripción, estado, prioridad |
| HU-04.2-user-stories-gherkin | proposed | Creación y versionado de HUs con Gherkin scenarios |
| HU-04.3-specs-designs | proposed | Specs, ADRs, alternativas descartadas, diagramas |
| HU-04.4-tasks-verification | proposed | Tasks, tests, sabotaje, cierre, checklist |
| HU-04.5-traceability | proposed | Trazabilidad REQ→HU→tasks, reportes de cobertura |
| HU-04.6-s3-storage | proposed | S3 integration para adjuntos de opsx, knowledge docs y assets |
