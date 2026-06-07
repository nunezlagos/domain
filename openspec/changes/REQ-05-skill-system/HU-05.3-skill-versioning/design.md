# Design: HU-05.3-skill-versioning

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| Storage | Snapshot completo (JSONB) | Delta/changeset | Lectura O(1), sin reconstrucción |
| Trigger | Handler-level (Go) | DB trigger | Más control, testing, logging |
| Diff engine | google/go-cmp con formateo custom | jsondiff | Probado, configurable, soporta JSON |
| Breaking detection | Structural JSON diff en required + types | Manual comparison | Automático, cubre todos los casos |
| Optimistic lock | version en WHERE clause | FOR UPDATE | Evita deadlocks, más simple |

## Alternativas descartadas

- **Deltas/changesets:** Ahorra espacio pero la lectura requiere reconstruir O(n). La tabla `skill_versions` no será massiva como para preocuparnos.
- **DB trigger:** Más difícil de testear y debugear. El handler-level nos da control explícito.
- **FOR UPDATE:** Overkill para este caso. Optimistic lock con version es suficiente.

## Diagrama

```
PATCH /api/skills/:id
  │
  ├─► Read current skill (with version as optimistic lock)
  ├─► Aplicar cambios
  ├─► Detectar breaking changes (comparar parameters.required, types)
  ├─► INSERT INTO skill_versions (skill_id, version, snapshot, changelog, breaking, author_id)
  ├─► UPDATE skills SET content=..., parameters=..., version = version + 1 WHERE id = $1 AND version = $old_version
  │     └─► Si RowsAffected == 0 → 409 Conflict (concurrent update)
  └─► 200 OK

POST /api/skills/:id/rollback { target_version }
  │
  ├─► Leer snapshot de target_version
  ├─► Si no existe → 404
  ├─► INSERT INTO skill_versions (...) AS nueva version
  ├─► UPDATE skills con snapshot, version++
  └─► 200 OK

GET /api/skills/:id/diff?from=2&to=5
  │
  ├─► Leer snapshots de ambas versiones
  ├─► google/go-cmp diff campo por campo
  ├─► Clasificar cambios: added, removed, modified
  ├─► Detectar breaking: required field added/removed, type changed
  └─► 200 OK con structured diff
```

### Modelo

```sql
CREATE TABLE skill_versions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id    UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    version     INT NOT NULL,
    snapshot    JSONB NOT NULL,
    changelog   TEXT,
    breaking    BOOLEAN NOT NULL DEFAULT false,
    author_id   UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(skill_id, version)
);

CREATE INDEX idx_skill_versions_skill ON skill_versions(skill_id, version DESC);

-- Campo en skills para pin
ALTER TABLE skills ADD COLUMN pinned_version INT;
```

## TDD plan

1. **TestUpdateCreaVersion:** PATCH → skill_versions tiene registro nuevo
2. **TestVersionIncrementada:** version pasa de N a N+1
3. **TestObtenerVersionEspecifica:** GET /versions/1 devuelve snapshot correcto
4. **TestListarVersiones:** GET /versions devuelve todos ordenados DESC
5. **TestPinAVersion:** pinned_version set, GET devuelve snapshot de esa versión
6. **TestUnpin:** pinned_version a NULL, GET devuelve latest
7. **TestRollback:** rollback a v3 → nueva v6 con mismo content que v3
8. **TestDiff:** diff entre versiones con cambios estructurales
9. **TestBreakingDetection:** cambio en required → breaking=true
10. **TestRollbackVersionInexistente:** 404
11. **TestOptimisticLock:** update concurrente → 409
12. **TestSabotaje:** Rollback crea versión correcta en skill_versions

## Riesgos y mitigación

- **Crecimiento de tabla:** Proyectar límite conservador de 100 versiones por skill. Job de cleanup opcional.
- **Breaking detection falsos positivos:** Un campo opcional agregado no es breaking. Solo contar required removidos o type changes.
- **Diff muy grande:** Limitar diff a 100KB. Si excede, devolver resumen + enlace a descarga.
