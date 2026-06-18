# Tasks: HU-28.8-timeafter-timertimer

> **Pre:** ninguno (refactor de leaks). Cambio de comportamiento: los
> timers se recolectan antes en loops largos (reducen presión de GC).

## Backend
- [x] **pc-001**: `internal/llm/retry/retry.go`: 2 retry loops (Complete +
  CompleteStream) usan `time.NewTimer(backoff)` + `t.Stop()` en branch
  ctx.Done().
- [x] **pc-002**: `internal/service/flow/saga.go`: 2 retry loops en
  RunCompensation usan NewTimer.
- [x] **pc-003**: `internal/service/flow/signals.go`: poll loop en
  WaitForSignal usa NewTimer.
- [x] **pc-004**: `internal/runner/flow/resume.go`: retry backoff entre
  steps usa NewTimer.
- [x] **pc-005**: `internal/runner/flow/retry.go`: retry delay entre
  attempts usa NewTimer.
- [x] **pc-006**: `internal/cache/distributed/cache.go`: exponential
  backoff usa NewTimer.

## NO tocado (deliberado)
- `time.After` en tests (esperas one-shot, no leak relevante).
- `time.After` en contextos que se ejecutan una sola vez (ej: connection
  timeout en stdio client).
- 28 usos restantes en archivos no-críticos (single-shot waits en
  select{}, no en loops).

## Verificación final
- [x] **vf-1**: código commiteado, sigue el patrón existente.
- [x] **vf-2**: state.yaml → implemented (este commit).
- [x] **vf-3**: REQ-28 state.yaml: 28.8 → implemented.

## Follow-up (opcional, no bloqueante)
- Si en el futuro se quiere ser más exhaustivo, los 28 time.After
  restantes pueden convertirse a NewTimer también. Pero el impacto
  en producción es bajo (cada uno se ejecuta una vez, no en loop).
- El linter `go vet` no detecta este patrón específicamente. Se
  podría agregar un linter custom (issue-35.X propuesto).
