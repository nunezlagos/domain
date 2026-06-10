# issue-09.4-retry-error-handling

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** ingeniero de automatización
**Quiero** configurar políticas de reintento por step (max_retries, backoff, retry_on) y manejo de errores (ignore, abort, retry, fallback)
**Para** que los flujos sean resilientes a fallos transitorios sin intervención manual y tengan comportamiento predecible ante errores permanentes

## Criterios de aceptación

### Escenario 1: Step con retry policy reintenta N veces con backoff exponencial

```gherkin
Dado que un step tiene configurado:
  """
  "retry": {
    "max_retries": 3,
    "backoff": "exponential",
    "initial_delay_ms": 1000
  }
  """
Cuando el step falla la primera vez
Entonces el sistema espera 1000ms antes de reintentar
Y el step se ejecuta nuevamente (intento 2)

Cuando el step falla el segundo intento
Entonces el sistema espera 2000ms (1000*2^1)
Y el step se ejecuta nuevamente (intento 3)

Cuando el step falla el tercer intento
Entonces el sistema espera 4000ms (1000*2^2)
Y el step se ejecuta nuevamente (intento 4, último)

Cuando el step falla el cuarto intento
Entonces el step se marca como `step_failed`
Y el flow aplica la política de error configurada
Y el resultado del step incluye `retry_count: 3`
Y el resultado incluye `errors: ["error1", "error2", "error3", "error4"]`
```

### Escenario 2: Retry con backoff fijo

```gherkin
Dado que un step tiene:
  """
  "retry": {
    "max_retries": 2,
    "backoff": "fixed",
    "fixed_delay_ms": 5000
  }
  """
Cuando el step falla
Entonces el sistema espera exactamente 5000ms antes del primer reintento
Y si falla nuevamente, espera otros 5000ms antes del segundo reintento
```

### Escenario 3: retry_on filtra qué errores reintentar

```gherkin
Dado que un step tiene:
  """
  "retry": {
    "max_retries": 2,
    "retry_on": ["timeout", "rate_limit"]
  }
  """
Cuando el step falla con error tipo `timeout`
Entonces el sistema reintenta (aplica retry policy)

Cuando el step falla con error tipo `validation_error`
Entonces el sistema NO reintenta (error no está en retry_on)
Y el step se marca como `step_failed` inmediatamente

Cuando retry_on está vacío o ausente
Entonces TODOS los errores son reintentables
```

### Escenario 4: ignore_and_continue continúa con valor por defecto

```gherkin
Dado que un step tiene `on_error: "ignore_and_continue"`
Y `default_on_error: {"fallback_value": "default"}`
Cuando el step falla (incluso después de agotar retries)
Entonces el flow NO se detiene
Y el step se marca como `step_completed_with_warnings`
Y el resultado del step se reemplaza con `default_on_error`
Y el flow continúa al siguiente step
Y el step anterior (dependencia) ve el fallback_value en lugar del resultado real
```

### Escenario 5: abort_flow detiene la ejecución inmediatamente

```gherkin
Dado que un step tiene `on_error: "abort_flow"`
Cuando el step falla
Entonces el flow pasa inmediatamente a estado `failed`
Y todos los steps en ejecución se cancelan
Y el resultado del flow contiene `error_step: "s2"`
Y el resultado contiene el mensaje de error original
```

### Escenario 6: fallback_step ejecuta un step alternativo

```gherkin
Dado que un step tiene:
  """
  "on_error": "fallback_step",
  "fallback_step": {
    "id": "s2_fallback",
    "type": "skill_call",
    "params": {"skill_slug": "handle-error"}
  }
  """
Cuando el step s2 falla
Entonces se ejecuta s2_fallback en lugar de s2
Y si s2_fallback tiene éxito, el flow continúa normalmente
Y el resultado de s2 en el contexto contiene `result` del fallback
Y el resultado incluye `fallback_used: true`

Cuando s2_fallback también falla
Entonces se aplica la política de error de s2_fallback (recursivo)
Y si no tiene política, se usa abort_flow por defecto
```

### Escenario 7: Dead Letter Queue para errores permanentes

```gherkin
Dado que un step falla permanentemente (agotó retries + sin política de recuperación)
Entonces el sistema crea un registro en la Dead Letter Queue (DLQ)
Y el DLQ contiene:
  """
  {
    "flow_run_id": "run-abc",
    "flow_slug": "customer-onboarding",
    "step_id": "s2",
    "error": "mensaje de error",
    "errors": ["intento1", "intento2", "intento3"],
    "failed_at": "2026-06-07T12:00:00Z"
  }
  ```

Cuando consulto la DLQ via GET /api/v1/dlq
Entonces recibo la lista de errores permanentes con paginación

Cuando elimino un registro de DLQ via DELETE /api/v1/dlq/:id
Entonces el registro se marca como resuelto
```

### Escenario 8: Política por defecto a nivel de flow

```gherkin
Dado que un flow tiene `default_step_error_policy: "abort_flow"`
Y un step NO tiene `on_error` configurado
Cuando el step falla
Entonces se aplica la política por defecto del flow (abort_flow)

Dado que el step tiene `on_error: "ignore_and_continue"`
Y el flow tiene `default_step_error_policy: "abort_flow"`
Cuando el step falla
Entonces se aplica la política del step (ignore_and_continue)
Y la política del step tiene prioridad sobre la del flow
```

## Análisis breve

- **Qué pide realmente:** Sistema de retry configurable por step con distintos backoff strategies, filtrado de errores reintentables, 4 políticas de manejo de error post-agotamiento, y Dead Letter Queue para errores permanentes.
- **Módulos sospechados:** `internal/flow/retry.go`, `internal/flow/error_handler.go`, `internal/models/dlq.go`, `internal/api/handlers/dlq.go`
- **Riesgos / dependencias:** Retry infinito si max_retries no está acotado. Fallback recursivo puede causar loops.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
