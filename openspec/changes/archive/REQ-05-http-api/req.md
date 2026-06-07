# REQ-05-http-api: API REST local (engram serve): endpoints para sessions, observations, search, timeline, prompts, context, export/import, stats, doctor, conflicts, project, sync/status.

**Estado:** activo
**Creado:** 2026-06-07

## Descripción

API REST local (engram serve): endpoints para sessions, observations, search, timeline, prompts, context, export/import, stats, doctor, conflicts, project, sync/status.

## Criterios de éxito

- Server HTTP escucha en `localhost:7437`
- 100% de los endpoints definidos en las HUs hijas responden correctamente
- Autenticación Bearer token protege rutas sensibles (DELETE, EXPORT, IMPORT, MIGRATE)
- Swagger/OpenAPI spec disponible en `/openapi.json`
- Health check responde en < 100ms
- Suite de tests de integración HTTP pasa completa

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-05.1 | proposed | HTTP endpoints para CRUD de sesiones con validación de integridad referencial |
| HU-05.2 | proposed | HTTP endpoints para CRUD de observaciones con detección de conflict candidates |
| HU-05.3 | proposed | HTTP endpoints para búsqueda full-text, timeline y context retrieval |
| HU-05.4 | proposed | HTTP endpoints para almacenar y recuperar prompts de usuario |
| HU-05.5 | proposed | HTTP endpoints para exportar/importar datos en formato JSON |
| HU-05.6 | proposed | HTTP endpoints para estadísticas, diagnóstico y health check |
| HU-05.7 | proposed | HTTP endpoints para resolución de proyecto y migración entre proyectos |
| HU-05.8 | proposed | HTTP endpoints para detección y resolución de conflictos |
| HU-05.9 | proposed | HTTP endpoint para estado de sincronización y autenticación Bearer |
