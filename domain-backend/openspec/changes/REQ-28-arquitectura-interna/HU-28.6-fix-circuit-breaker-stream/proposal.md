# Proposal: HU-28.6-fix-circuit-breaker-stream

## Intención

Un cambio de 1 línea: `_ = sawError` → `if sawError { cb.recordFailure() }`. Cierra el gap donde el breaker ignoraba errores mid-stream.

## Scope

**Incluye:** Solo `breaker.go:171`.

**No incluye:** Refactor del breaker, cambio de semántica de half-open, ni otros métodos.
