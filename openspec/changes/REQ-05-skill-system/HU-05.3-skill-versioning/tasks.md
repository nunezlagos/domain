# Tasks: HU-05.3-skill-versioning

## Backend

- [ ] Crear migración para tabla `skill_versions` + índices
- [ ] Agregar campo `pinned_version` a tabla `skills`
- [ ] Implementar lógica de snapshot en PATCH handler (insert en skill_versions)
- [ ] Implementar handler GET /api/skills/:id/versions
- [ ] Implementar handler GET /api/skills/:id/versions/:version
- [ ] Implementar handler POST /api/skills/:id/pin
- [ ] Implementar handler POST /api/skills/:id/rollback
- [ ] Implementar handler GET /api/skills/:id/diff
- [ ] Implementar detección de breaking changes (comparación de schemas)
- [ ] Implementar optimistic lock para updates concurrentes
- [ ] Implementar resolución de versión (pinned vs latest) en lecturas

## Frontend

- [ ] N/A (solo API)

## Tests

- [ ] Test unitario: inserción de snapshot en skill_versions
- [ ] Test unitario: breaking change detection (requerido, tipos)
- [ ] Test unitario: diff JSON entre snapshots
- [ ] Test integración: ciclo update 5 veces → listar → obtener versión específica
- [ ] Test integración: pin a versión → verificar lecturas usan snapshot
- [ ] Test integración: rollback → nueva versión con contenido exacto
- [ ] Test integración: diff entre versiones con cambios
- [ ] Test integración: optimistic lock → 409 en concurrencia
- [ ] Sabotaje: rollback a versión inexistente → 404

## Cierre

- [ ] Verificación manual con curl
- [ ] Suite verde
