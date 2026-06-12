# Proposal: HU-28.2-constructors-validation

## Intención

Reemplazar struct literals públicos por constructores canónicos con validación temprana. Strangler Fig: fields públicos se mantienen con doc `Deprecated` hasta que no haya consumidores legacy.

## Scope

**Incluye:** Todos los services construidos en `cmd/domain/main.go`. Validación de nil para pool, audit, repo (cuando aplica). Fields privados donde sea posible.

**No incluye:** Cambios en la API pública de los services más allá del constructor. Refactor de los Store structs internos (DLQStore, etc.).
