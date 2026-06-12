# Design: HU-28.6-fix-circuit-breaker-stream

## Cambio

```go
// breaker.go:168-172
// Antes:
    sawError := false
    // ... goroutine que setea sawError ...
    _ = sawError // dead code

// Después:
    sawError := false
    // ... goroutine que setea sawError ...
    if sawError {
        cb.recordFailure()
    }
```

## Consideraciones

- `recordFailure()` es thread-safe (usa `sync.Mutex` del breaker).
- El breaker tiene un umbral de fallos antes de abrirse. Este cambio puede hacer que el breaker se abra antes que antes — que es el comportamiento correcto y esperado.
- Tests existentes de breaker pueden necesitar ajuste si asumían que mid-stream errors no contaban.

## TDD

1. Red: test que crea un breaker con threshold=1, stream con error, verifica que el breaker NO está abierto (falla porque recordFailure no se llama).
2. Green: agregar `if sawError { cb.recordFailure() }`.
3. Sabotaje: cambiar `recordFailure` por `recordSuccess`, test falla.
