# Proposal: issue-09.4-retry-error-handling

## Intención

Implementar políticas de reintento configurables por step (max_retries, backoff exponential/fixed, retry_on filters) y estrategias de manejo de error post-agotamiento (ignore_and_continue, abort_flow, retry_step, fallback_step). Los errores permanentes se persisten en una Dead Letter Queue para revisión manual.

## Scope

**Incluye:**
- Modelo `RetryPolicy` con campos: max_retries, backoff (exponential|fixed), initial_delay_ms, fixed_delay_ms, retry_on ([]string)
- Ejecutor de retry con backoff calculation
- 4 políticas de error: ignore_and_continue, abort_flow, retry_step, fallback_step
- Política por defecto a nivel de flow + override por step
- Dead Letter Queue CRUD: create on permanent failure, list, delete (resolve)
- Integración con flow runner: antes de fallar un step, intenta retry policy

**Excluye:**
- Notificaciones cuando un step cae a DLQ (futuro)
- Circuit breaker pattern (futuro si hay muchos errores consecutivos)
- Reintento manual desde DLQ (futuro)

## Enfoque técnico

- `RetryPolicy` struct con método `ShouldRetry(error, attempt int) (bool, time.Duration)`
- `BackoffCalculator` interface con implementaciones `ExponentialBackoff` y `FixedBackoff`
- `ErrorHandler` que recibe: step, error, policy, flowContext → decide acción
- Integración en flow runner: loop de retry alrededor de StepRunner.Run()
- DLQ como tabla separada en Postgres con CRUD simple
- La política por defecto del flow se almacena en el campo `default_step_error_policy` del modelo Flow

## Riesgos

- Retry infinito: siempre validar max_retries > 0 y default 3 si no se especifica
- Fallback recursivo: limitar profundidad de fallback a 3 niveles
- ignore_and_continue con default_on_error vacío: requerir default_on_error
- DLQ puede crecer sin límite: agregar TTL (30 días) y límite por proyecto

## Testing

- Unit: todas las combinaciones de backoff (exponential, fixed, delays correctos)
- Unit: retry_on filter (match, no match, empty)
- Unit: cada política de error produce la acción esperada
- Unit: fallback_step se ejecuta y su resultado reemplaza al original
- Unit: retry_count se incrementa correctamente
- Integration: retry + eventual success
- Integration: retry + eventual failure → DLQ
- Sabotaje: retry sin límite → test de cota máxima falla
