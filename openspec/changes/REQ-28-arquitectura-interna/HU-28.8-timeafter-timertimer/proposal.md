# Proposal: HU-28.8-timeafter-timertimer

## Intención

Replace `time.After` con `time.NewTimer` + `defer timer.Stop()` en loops de retry. Zero cambio de comportamiento, solo eficiencia de memoria bajo carga.

## Scope

**Incluye:** Ambos lugares en `llm/retry/retry.go` y el lugar en `mcp/server/resilience.go`.

**No incluye:** Otros usos de `time.After` en tests (son tests, no prod).
