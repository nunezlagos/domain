# Tasks: issue-05.3-skill-versioning

## Backend

- [x] Crear migración para tabla `skill_versions` + índices
- [x] Agregar campo `pinned_version` a tabla `skills`
- [x] Implementar lógica de snapshot en PATCH handler (insert en skill_versions)
- [x] Implementar handler GET /api/skills/:id/versions
- [x] Implementar handler GET /api/skills/:id/versions/:version
- [x] Implementar handler POST /api/skills/:id/pin
- [x] Implementar handler POST /api/skills/:id/rollback
- [x] Implementar handler GET /api/skills/:id/diff
- [x] Implementar detección de breaking changes (comparación de schemas)
- [x] Implementar optimistic lock para updates concurrentes
- [x] Implementar resolución de versión (pinned vs latest) en lecturas

## Frontend

- [x] N/A (solo API)

## Tests

- [x] Test unitario: inserción de snapshot en skill_versions
- [x] Test unitario: breaking change detection (requerido, tipos)
- [x] Test unitario: diff JSON entre snapshots
- [x] Test integración: ciclo update 5 veces → listar → obtener versión específica
- [x] Test integración: pin a versión → verificar lecturas usan snapshot
- [x] Test integración: rollback → nueva versión con contenido exacto
- [x] Test integración: diff entre versiones con cambios
- [x] Test integración: optimistic lock → 409 en concurrencia
- [x] Sabotaje: rollback a versión inexistente → 404

## Cierre

- [x] Verificación manual con curl
- [x] Suite verde
