# Proposal: issue-21.6-org-schema-decommission

## Intención

Eliminar definitivamente el plumbing multi-tenant a nivel DB y app una vez colapsado el
surface (issue-21.5): columna `organization_id` en 54 tablas, RLS por `current_org_id()`,
tabla `organizations` y satélites, y todo el threading de `org_id` en queries Go.

## Scope

**Incluye:**
- Migración(es) destructiva(s) que dropean RLS org, columnas `organization_id`, función
  `current_org_id()`, trigger cross-org, tabla `organizations` y satélites.
- Refactor de ~658 refs Go (queries `WHERE organization_id`, structs con `OrganizationID`,
  `SET LOCAL app.current_org_id` en middleware, ctxkeys org).
- Ajuste de seeds, tests (~189 refs) y SDKs (campos org en modelos wire).

**No incluye:**
- Cambios funcionales de producto distintos a la remoción de org.

## Enfoque técnico — ejecución por FASES (cada una deployable y verificable)

> **Pre-requisito de toda la HU:** backup verificado (pgBackRest, ver docs/runbooks/restore.md)
> y ventana de mantenimiento. Es destructivo e irreversible sobre datos.

### Fase A — App deja de depender del GUC y de RLS por org
1. Middleware: dejar de ejecutar `SET LOCAL app.current_org_id` (mantener user_id si aplica).
2. Migración: `DISABLE ROW LEVEL SECURITY` + `DROP POLICY *_org_isolation` en las ~20 tablas.
3. Verificar app verde sin RLS por org (los `WHERE organization_id` siguen, redundantes pero válidos).

### Fase B — Quitar threading org_id de queries Go (por paquete)
4. Por paquete (service/*, mcp/*, runner/*, etc.): remover args `orgID`, cláusulas
   `WHERE organization_id = $N`, campos `OrganizationID` de structs internos.
5. `go build ./...` verde tras cada paquete; deploy incremental.

### Fase C — Drop de columnas y tablas
6. Migración: `ALTER TABLE ... DROP COLUMN organization_id` en las 54 tablas (preserva filas).
7. Migración: drop función `current_org_id()`, trigger cross-org, tabla `organizations` + satélites.
8. Quitar `current_user_id()`/GUC si quedó sin uso.

### Fase D — Limpieza de periferia
9. SDKs: remover `Organization` model + campo `organization_id` en Project/Observation (wire).
10. Seeds/fixtures/tests: remover org. Docs (rls.md, etc.).

## Riesgos

- **Irreversible sobre datos:** backup obligatorio + dry-run en staging.
- **Pérdida temporal de aislamiento:** aceptable en single-org (todo el dataset es la org).
- **Queries que rompen:** `go build ./...` + suite de integración por paquete.
- **Orden de drops por FK:** dropear columnas/policies antes de la tabla `organizations`.

## Testing

- Dry-run completo en entorno de staging con copia de datos de prod.
- Conteo de filas pre/post por tabla (debe preservarse).
- Suite de integración (`*_integration_test.go`) verde tras cada fase.
