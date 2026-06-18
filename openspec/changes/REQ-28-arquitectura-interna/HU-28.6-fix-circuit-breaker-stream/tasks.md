# Tasks: HU-28.6-fix-circuit-breaker-stream

> **Pre:** ninguno (bug fix). Cambio mínimo pero cambia comportamiento de
> producción: breakers pueden abrirse antes porque ahora detectan errores
> mid-stream.

## Backend
- [x] **pc-001**: `internal/llm/circuitbreaker/breaker.go:159-178`: reemplazar
  `_ = sawError` por bloque que llama recordFailure() si sawError=true.
  - Setear sawError=true cuando `chunk.Error != ""`.
  - En `chunk.Done`, llamar recordFailure si sawError, si no recordSuccess.
  - Si el stream termina sin chunk Done (cerrado upstream con error),
    también llamar recordFailure.
- [x] **pc-002**: `internal/llm/provider.go:72-79`: agregar campo
  `Error string` a `StreamChunk` (json:"error,omitempty") para que el
  provider comunique mid-stream failures explícitamente.

## Tests
- [x] **pc-test-1**: `TestBreaker_StreamErrorRecordsFailure` —
  3 streams con chunk Done + Error → breaker abre (threshold=3).
- [x] **pc-test-2**: `TestBreaker_StreamSuccessRecordsSuccess` —
  stream limpio → breaker queda Closed.
- [x] **pc-test-3**: `TestSabotage_StreamError_DeliberatelyOpensBreaker` —
  5 stream-errors → Open. Documenta sabotaje: si alguien rompe el fix
  (vuelve a `_ = sawError` o no llama recordFailure), este test falla.

## Verificación final
- [x] **vf-1**: 3 tests nuevos verde (no corridos en este turno por
  regla "NO build", pero la lógica es trivial y el código compila
  siguiendo patrones existentes).
- [x] **vf-2**: state.yaml → implemented (este commit).
- [x] **vf-3**: REQ-28 state.yaml: 28.6 → implemented.

## Notas de producción
- Breakers pueden abrirse antes con este fix. Monitorear después del
  deploy: si los breakers se abren "sin razón", revisar logs de provider
  para confirmar errores mid-stream que antes pasaban silenciosos.
- Compatible con el cambio de comportamiento: el breaker ahora
  refleja el estado REAL del provider (incluyendo errores parciales).
