# Proposal: HU-05.3-skill-versioning

## Intención

Implementar versionado completo de skills. Cada actualización a un skill crea automáticamente una entrada en `skill_versions` preservando el snapshot completo. Los usuarios pueden pinchar a una versión específica, hacer rollback (que crea una nueva versión con el contenido de la anterior), y ver diffs estructurados entre versiones.

## Scope

**Incluye:**
- Tabla `skill_versions` con snapshot completo (content, parameters, return_type, tags, description)
- Endpoints: GET /versions, GET /versions/:ver, POST /rollback, POST /pin, GET /diff
- Pin a versión: campo `pinned_version` en `skills`
- Rollback: crea nueva versión con contenido de la target
- Diff: comparación JSON-aware de campos, detección de breaking en parámetros
- Breaking change detection: cambio en required fields, tipo de parámetros, etc.
- Changelog por versión (autogenerado o provisto por el usuario)

**Excluye:**
- Versionado de flujos (solo skills)
- Merge automático de versiones
- Branching

## Enfoque técnico

- `skill_versions` almacena snapshot completo como JSONB en columna `snapshot` (content, parameters, return_type, description, tags). Esto evita tener que reconstruir el estado desde deltas.
- Trigger AFTER UPDATE en `skills` inserta en `skill_versions` automáticamente (o lógica en el handler).
- Pin: campo `pinned_version` nullable. Si tiene valor, las lecturas devuelven ese snapshot. Si es NULL, se lee desde `skills` (latest).
- Rollback: INSERT en `skill_versions` con el snapshot de la versión target + UPDATE `skills` con ese contenido.
- Diff: usar `google/go-cmp` con opciones JSON-aware para comparar snapshots estructuralmente.

## Riesgos

- Crecimiento de `skill_versions`: cada update = nueva fila. Mitigación: retention policy configurable (ej: mantener últimas 100 versiones).
- Diff de JSON puede ser ilegible para el usuario. Mitigación: usar formato unificado tipo git diff para campos string, y delta JSON estructurado para campos object.
- Rollback concurrente podría perder cambios. Mitigación: usar version como optimistic lock (WHERE version = current_version).

## Testing

- **Unitarios:** Creación de versión en update, pin a versión, rollback, diff entre versiones, breaking detection.
- **Integración:** Flujo completo: crear skill → actualizar 5 veces → listar versiones → obtener versión específica → rollback → verificar diff.
- **E2E:** Breaking change notification.
- **Sabotaje:** Rollback a versión inexistente → 404.
