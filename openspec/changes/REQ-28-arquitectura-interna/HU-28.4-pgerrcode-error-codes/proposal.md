# Proposal: HU-28.4-pgerrcode-error-codes

## Intención

Reemplazar `strings.Contains(err.Error(), "duplicate key")` por `errors.As` + `pgerrcode` en toda la codebase. Cada cambio es atómico por archivo.

## Scope

**Incluye:** Todos los matches de `strings.Contains(err.Error(), ...)` contra errores de Postgres.

**No incluye:** Migración de otros strings contiene que no sean errores de DB (ej: validaciones de input).
