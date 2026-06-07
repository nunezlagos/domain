# Design: HU-09.4-retry-error-handling

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Backoff calculation | Interface + strategies | Switch case (interface es extensible para futuros backoffs como linear, jitter) |
| Error action decision | Enum + switch en ErrorHandler | Strategy pattern por política (switch es suficiente para 4 variantes) |
| DLQ storage | Tabla Postgres separada | Archivo JSON, S3 (DB permite query consistente con el resto del sistema) |
| retry_on matching | Prefix match (strings.HasPrefix) | Regex completo (prefix es suficiente para errores con namespace: "timeout.*", "rate_limit.*") |

## Alternativas descartadas

- **Jitter en backoff**: No incluido en MVP para mantener predictibilidad. Se puede agregar como opción "exponential_with_jitter" después.
- **Circuit breaker**: Fuera de scope. Si un step falla consistentemente, el retry policy lo maneja; circuit breaker sería optimización para no golpear servicios caídos.
- **DLQ con cola externa (RabbitMQ/SQS)**: Overkill para MVP. Tabla Postgres + worker de reintento manual es suficiente.

## Diagrama

```
Flujo de ejecución de step con retry:

StepRunner.Run()
       │
       ▼
┌──────────────────┐
│  Ejecutar step   │
└──────┬───────────┘
       │
       ├── éxito ──► step_completed
       │
       ▼ error
┌──────────────────┐
│ ¿retry_on match? │── no ──► ┌──────────────────┐
└──────┬───────────┘          │  Aplicar política │
       │ si                   │  de error         │
       ▼                      └──────────────────┘
┌──────────────────┐
│ attempt++         │
│ ¿attempt ≤ max?   │── no ──► ┌──────────────────┐
└──────┬───────────┘          │  Aplicar política │
       │ si                   │  de error         │
       ▼                      └──────────────────┘
┌──────────────────┐
│ Calcular delay    │
│ Esperar N ms      │
└──────┬───────────┘
       │
       └──► Loop a StepRunner.Run()
```

Políticas de error (post-agotamiento de retries):

```
on_error: "ignore_and_continue"
  → step status = step_completed_with_warnings
  → result = default_on_error (required)
  → flow continúa

on_error: "abort_flow"
  → step status = step_failed
  → flow status = failed
  → todos los steps activos cancelados
  → error guardado en domain_flow_run.error

on_error: "fallback_step"
  → ejecuta fallback_step definido en el step
  → fallback tiene su propia retry/error policy
  → máximo 3 niveles de profundidad de fallback anidado

on_error: "retry_step" (no configurado)
  → idéntico a tener retry policy, pero ocurre post-agotamiento
  → reintenta el mismo step N veces más
```

DLQ schema:
```sql
CREATE TABLE dead_letter_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id),
    flow_run_id UUID NOT NULL REFERENCES flow_runs(id),
    flow_slug VARCHAR(255) NOT NULL,
    step_id VARCHAR(255) NOT NULL,
    attempt_count INT NOT NULL,
    last_error TEXT NOT NULL,
    all_errors JSONB NOT NULL,
    failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## TDD plan

1. **Red:** Test `TestExponentialBackoff_Delays` — delays correctos: 1s, 2s, 4s, 8s
2. **Green:** Implementar ExponentialBackoff
3. **Red:** Test `TestFixedBackoff_Delays` — delay constante
4. **Green:** Implementar FixedBackoff
5. **Red:** Test `TestRetryPolicy_ShouldRetry` — retry_on match y no match
6. **Green:** Implementar ShouldRetry con prefix match
7. **Red:** Test `TestErrorHandler_IgnoreAndContinue` — continúa con fallback
8. **Green:** Implementar ignore_and_continue
9. **Red:** Test `TestErrorHandler_AbortFlow` — flow falla
10. **Green:** Implementar abort_flow
11. **Red:** Test `TestErrorHandler_FallbackStep` — fallback se ejecuta
12. **Green:** Implementar fallback_step
13. **Red:** Test `TestDLQ_Create` — error permanente crea DLQ entry
14. **Green:** Implementar DLQ repository
15. **Sabotaje:** Retry sin límite de max_retries → test de cota falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Retry infinito | Baja | Alto | max_retries default 3, máximo configurable 10 |
| Fallback recursivo infinito | Baja | Alto | Máximo 3 niveles de profundidad de fallback |
| DLQ sin mantenimiento | Media | Bajo | TTL de 30 días, cleanup job automático |
| ignore_and_continue silencia errores importantes | Media | Medio | Warning en logs + step marcado con_warnings (visible en UI) |
