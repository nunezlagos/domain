# issue-09.5-subflows-composition

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** ingeniero de automatización
**Quiero** que un flow pueda ejecutar otro flow como un paso (sub-flow), con paso de contexto padre→hijo y soporte para ejecución paralela de sub-flows
**Para** componer pipelines complejos reutilizando flows existentes como bloques modulares

## Criterios de aceptación

### Escenario 1: Un flow ejecuta otro flow como sub-flow

```gherkin
Dado que existe un flow "email-notifier" con slug "email-notifier"
Y existe un flow "onboarding" que tiene un paso sub_flow:
  """
  {
    "id": "send_email",
    "type": "sub_flow",
    "params": {
      "flow_slug": "email-notifier",
      "input": {
        "to": "{{steps.validate_email.result.email}}",
        "template": "welcome"
      }
    }
  }
  """
Cuando ejecuto el flow "onboarding"
Y llego al paso send_email
Entonces el sistema inicia una nueva ejecución del flow "email-notifier"
Y le pasa el input `{"to": "...", "template": "welcome"}`
Y espera a que el sub-flow complete
Y cuando el sub-flow completa exitosamente
Entonces el paso send_email se marca como `step_completed`
Y el resultado del sub-flow se almacena en `steps.send_email.result.flow_run_id`
Y el resultado incluye `steps.send_email.result.output` con la salida del sub-flow
```

### Escenario 2: Sub-flow fallido propaga error

```gherkin
Dado que ejecuto un flow con un paso sub_flow
Y el sub-flow falla
Entonces el paso sub_flow se marca como `step_failed`
Y el error contiene el mensaje de error del sub-flow
Y el flow padre aplica su política de error (retry, abort, etc.)
```

### Escenario 3: Múltiples sub-flows en paralelo

```gherkin
Dado que un flow tiene un paso parallel que contiene 2 sub-flows:
  """
  {
    "id": "parallel_subflows",
    "type": "parallel",
    "params": {
      "branches": [
        {"id": "sf1", "type": "sub_flow", "params": {"flow_slug": "data-enrichment", "input": {"id": "{{input.id}}"}}},
        {"id": "sf2", "type": "sub_flow", "params": {"flow_slug": "risk-analysis", "input": {"id": "{{input.id}}"}}}
      ]
    }
  }
  """
Cuando el flow ejecuta parallel_subflows
Entonces ambos sub-flows se lanzan concurrentemente
Y se espera a que ambos completen
Y los resultados están disponibles en `steps.parallel_subflows.results[0]` y `[1]`
```

### Escenario 4: Context passing padre → hijo → padre

```gherkin
Dado que un flow padre tiene en su contexto `steps.s1.result = {"user_id": 123}`
Y el sub-flow recibe input `{"user_id": "{{steps.s1.result.user_id}}"}`
Cuando el sub-flow se ejecuta
Entonces recibe `{"user_id": 123}` como input inicial

Cuando el sub-flow completa con output `{"analysis": "approved", "score": 95}`
Entonces el flow padre tiene acceso a ese output vía `steps.subflow_step.result.output`

Dado que el flow padre tiene un step después del sub-flow
Entonces ese step puede usar `{{steps.subflow_step.result.output.score}}`
```

### Escenario 5: Sub-flow referenciado no existe

```gherkin
Dado que un flow tiene un paso sub_flow con `flow_slug: "non-existent"`
Cuando ejecuto el flow
Y llego al paso sub_flow
Entonces el paso falla con error "Sub-flow 'non-existent' not found"
Y el flow aplica su política de error
```

### Escenario 6: Detección de sub-flow circular

```gherkin
Dado que el flow "flow-a" tiene un paso sub_flow que referencia "flow-b"
Y el flow "flow-b" tiene un paso sub_flow que referencia "flow-a"
Cuando intento ejecutar "flow-a"
Y llego al paso sub_flow
Entonces el sistema detecta la referencia circular
Y el paso falla con error "Circular sub-flow reference detected: flow-a → flow-b → flow-a"
```

### Escenario 7: Reusable flow templates con parámetros

```gherkin
Dado que existe un flow "notification" con slug "notification"
Y el flow usa `{{input.recipient}}` y `{{input.message}}` en sus steps
Y otro flow "order-confirmation" tiene un paso sub_flow:
  """
  "params": {
    "flow_slug": "notification",
    "input": {
      "recipient": "{{steps.get_order.result.email}}",
      "message": "Tu orden {{steps.get_order.result.order_id}} está lista"
    }
  }
  """
Cuando ejecuto "order-confirmation"
Y el sub-flow se ejecuta
Entonces el sub-flow recibe los parámetros mapeados
Y ejecuta correctamente usando esos valores como input
```

### Escenario 8: Listar flows que usan un flow como sub-flow

```gherkin
Dado que el flow "notification" es usado como sub-flow por "order-confirmation" y "signup"
Cuando consulto GET /api/v1/flows/notification/parents
Entonces recibo:
  """
  {
    "data": [
      {"slug": "order-confirmation", "name": "Order Confirmation"},
      {"slug": "signup", "name": "User Signup"}
    ]
  }
  """
```

## Análisis breve

- **Qué pide realmente:** Composición de flows: un flow puede ser step de otro. Contexto padre→hijo. Ejecución paralela de sub-flows. Detección de referencias circulares. Templates reutilizables.
- **Módulos sospechados:** `internal/flow/subflow.go`, `internal/flow/step_types/sub_flow.go` (ya esbozado en issue-09.2), `internal/flow/runner.go`
- **Riesgos / dependencias:** Referencias circulares. Profundidad de anidamiento excesiva. Dependencia de issue-09.2 (SubFlowRunner) y issue-09.3 (flow runner).
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
