# Tasks: HU-09.4-retry-error-handling

## Backend

- [ ] Implementar `BackoffCalculator` interface + `ExponentialBackoff` + `FixedBackoff`
- [ ] Implementar modelo `RetryPolicy` en `internal/models/retry.go`
- [ ] Implementar `RetryExecutor` que maneja el loop de reintento
- [ ] Implementar `ErrorHandler` con las 4 políticas de error
- [ ] Integrar retry en flow runner (loop de reintento antes de fallar)
- [ ] Integrar error handler en flow runner (acción post-agotamiento)
- [ ] Agregar campo `default_step_error_policy` al modelo Flow
- [ ] Crear migración SQL para tabla `dead_letter_queue`
- [ ] Implementar `DLQRepository` con Create, List, Delete (resolve)
- [ ] Crear handler REST: GET /api/v1/dlq (list con paginación)
- [ ] Crear handler REST: DELETE /api/v1/dlq/:id (resolve)
- [ ] Agregar validación: fallback_step requiere fallback_step definido
- [ ] Agregar validación: ignore_and_continue requiere default_on_error
- [ ] Agregar límite de 3 niveles de profundidad de fallback
- [ ] Agregar límite máximo de max_retries (10)

## Tests

- [ ] Test unitario: ExponentialBackoff delays correctos (1, 2, 4, 8s)
- [ ] Test unitario: FixedBackoff delays constantes
- [ ] Test unitario: ShouldRetry con retry_on match (timeout)
- [ ] Test unitario: ShouldRetry con retry_on no match (validation_error)
- [ ] Test unitario: ShouldRetry con retry_on vacío (todos reintentan)
- [ ] Test unitario: RetryExecutor max_retries alcanzado
- [ ] Test unitario: RetryExecutor éxito en intento 2 de 3
- [ ] Test unitario: ignore_and_continue con default_on_error
- [ ] Test unitario: abort_flow detiene ejecución
- [ ] Test unitario: fallback_step se ejecuta y reemplaza resultado
- [ ] Test unitario: fallback_step recursivo máximo 3 niveles
- [ ] Test unitario: política de step sobreescribe política del flow
- [ ] Test unitario: DLQ Create registra error permanente
- [ ] Test unitario: DLQ List con paginación
- [ ] Test de integración: step con retry → eventual success
- [ ] Test de integración: step con retry → eventual failure → DLQ
- [ ] Sabotaje: max_retries sin límite → test de cota falla

## Cierre

- [ ] Verificación manual: step con retry_on timeout, falla con validation → no retry
- [ ] Verificación manual: DLQ aparece después de error permanente
- [ ] Suite verde
