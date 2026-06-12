# Proposal: HU-28.5-fix-ignored-errors

## Intención

Dejar de tragar errores críticos. Logging para los best-effort (audit, JSON encode), propagación para los que pueden fallar aguas arriba (marshal).

## Scope

**Incluye:** Todos los `_ =` que ignoran errores en categorías: HTTP response, auditoría, marshaling, rollback.

**No incluye:** Errores de `defer` cleanup que ya están correctamente manejados. Errores de `io.Copy` en streaming (caso especial donde no siempre es error).
