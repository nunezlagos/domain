# Proposal: issue-09.7-workflow-versioning

## Intención

Versionar flows con immutability per-version: cada save crea draft, publish la activa, runs en vuelo terminan con su versión congelada. Permite editar flows en prod sin romper runs largos.

## Scope

**Incluye:**
- Tabla `flow_versions` con spec immutable
- Lifecycle draft → published → deprecated
- `flow_runs.flow_version_id` apunta a versión congelada
- Endpoint publish/deprecate
- Diff JSON entre versiones (json-patch)
- Detección heurística de breaking changes

**No incluye:**
- Migración automática de runs en vuelo (queda como manual)
- Rollback automático (deprecate + re-publish anterior es el rollback)

## Enfoque técnico

1. `flow_versions` con spec JSONB completo + checksum SHA256
2. PATCH /flows/:id NO modifica versión actual, crea draft
3. Engine lee version_id del run, no current de flow
4. Diff con `evanphx/json-patch`
5. Breaking change detector: walks AST y compara estructuras

## Riesgos

- Storage bloat: cron archive versiones deprecated >90 días sin runs
- Confusion API: documentar bien "current" vs "published"
- Breaking change heurística falla edge cases: solo warning, no enforcement

## Testing

- Save crea draft
- Publish activa
- Run en vuelo no se afecta por nuevo publish
- Invoke versión específica
- Deprecate bloquea nuevas runs
- Diff json-patch correcto
- Breaking change detector flagsl
