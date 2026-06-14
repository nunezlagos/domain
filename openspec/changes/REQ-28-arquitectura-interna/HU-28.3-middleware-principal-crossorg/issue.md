# HU-28.3-middleware-principal-crossorg

**Origen:** `REQ-28-arquitectura-interna`
**Prioridad tentativa:** alta
**Tipo:** refactor

## Historia de usuario

**Como** desarrollador de Domain
**Quiero** que el Principal (orgID, userID) se inyecte en el context como value objects por un middleware, y que el cross-org guard sea un helper reutilizable
**Para** eliminar ~30 repeticiones de `p, _ := principal(r); if p == nil` y ~15 repeticiones del patrón `if errors.Is(err, X.ErrNotFound) || (err == nil && out.OrgID != p.OrgID)` que son fuente de bugs de seguridad si se olvida en un handler nuevo

## Contexto

Hoy:
1. Cada handler hace `p, _ := principal(r); if p == nil { ... }` (repetido ~30 veces)
2. Cada handler que devuelve un recurso chequea `out.OrganizationID.String() != p.OrganizationID` a mano (~15 veces)
3. `OrganizationID` y `UserID` viajan como strings crudos — cada handler los parsea con `uuid.Parse()`

El patrón correcto: un middleware post-auth que setea `OrgID uuid.UUID` y `UserID uuid.UUID` en el context. Un helper `authorizeOrg(ctx, orgID) error` en el package handler. Value objects tipados eliminan el parsing repetitivo.

## Criterios de aceptación

### Escenario 1: Middleware inyecta OrgID/UserID tipados

```gherkin
Dado un request autenticado
Cuando pasa por el middleware de principal
Entonces ctx tiene OrgID (uuid.UUID) y UserID (uuid.UUID) como value objects
Y ningún handler necesita llamar a principal(r) ni uuid.Parse
```

### Escenario 2: Cross-org helper

```gherkin
Dado un handler que obtiene un recurso por ID
Cuando el recurso pertenece a otra organización
Entonces `a.authorizeOrg(ctx, resource.OrgID)` retorna error 404
Y el handler no necesita escribir el if manualmente
```

### Escenario 3: Handler nuevo sin authorizeOrg

```gherkin
Dado un handler nuevo que omite authorizeOrg
Cuando devuelve un recurso de otra org
Entonces el middleware de principal NO bloquea (no es su responsabilidad)
Pero la RLS de Postgres (issue-25.14) bloquea el leak si el query usa la tx del contexto
```

## Análisis breve

- **Qué pide:** Middleware que setea `OrgID`/`UserID` en context post-auth. Helper `authorizeOrg` en handler. Migración gradual de handlers.
- **Módulos afectados:** `internal/api/handler/api.go` (helper), `internal/api/middleware/` (nuevo middleware principal), todos los handlers existentes
- **Esfuerzo tentativo:** M (2-3 días)
- **Dependencias:** HU-28.1, HU-28.2 (para consistencia). Puede implementarse en paralelo si se toca solo handler/api.go.
