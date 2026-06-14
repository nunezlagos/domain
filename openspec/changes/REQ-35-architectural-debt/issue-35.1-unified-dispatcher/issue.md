# issue-35.1-unified-dispatcher

**Origen:** `REQ-35-architectural-debt`
**Prioridad tentativa:** baja
**Tipo:** refactor (deuda técnica)

## Historia de usuario

**Como** developer de domain manteniendo el código de dispatch (cron, webhook, MCP)
**Quiero** tener UN SOLO dispatcher con la lógica de "qué target type → qué runner" centralizada
**Para** no tener que mantener 3 implementaciones paralelas que divergen silenciosamente (bug típico: agregar un step_type nuevo y olvidarse de uno de los dispatchers)

## Criterios de aceptación

### Escenario 1: Interface única `Dispatch`

```gherkin
Dado que existe `internal/dispatch.Dispatcher` con:
  type Dispatcher struct {...}
  func (d *Dispatcher) Dispatch(ctx, req DispatchRequest) (DispatchResult, error)
  type DispatchRequest struct {
    OrgID uuid.UUID
    Source string  // "cron" | "webhook" | "mcp" | "manual"
    TargetType string  // "flow" | "agent" | "skill"
    TargetID uuid.UUID
    Inputs json.RawMessage
    TriggeredBy *uuid.UUID  // user id si aplica
  }
Cuando un cron, webhook, o MCP server quiere ejecutar un target
Entonces TODOS llaman a `dispatcher.Dispatch(ctx, req)` con el mismo shape
Y la lógica de "qué runner usar" vive en UN solo lugar
```

### Escenario 2: Los 3 call-sites usan el dispatcher

```gherkin
Dado que el código actual tiene 3 dispatchers (cron, webhook, mcp)
Cuando refactorizamos
Entonces:
  - cronsched.dispatchSync → dispatcher.Dispatch(Source: "cron", ...)
  - webhook.dispatchWebhook → dispatcher.Dispatch(Source: "webhook", ...)
  - mcp.handleFlowRun + mcp.handleAgentRun + mcp.handleSkillExecute → todos dispatch.Dispatch
Y el código viejo de dispatch se ELIMINA (no queda como dead code)
Y un test verifica que NO existen las funciones viejas (grep)
```

### Escenario 3: Mismo target_type se ejecuta igual independientemente del source

```gherkin
Dado que existe un flow "F" y un user lo dispara via:
  - MCP tool domain_flow_run con id=F
  - Webhook con payload {action: "run_flow", flow_id: F}
  - Cron que tiene step {type: "run_flow", target: F}
Cuando los 3 disparan
Entonces el flow F se ejecuta IDÉNTICAMENTE:
  - Mismo flow runner
  - Mismas métricas
  - Mismo audit log (con source diferenciado: "cron" | "webhook" | "mcp")
  - Mismo timeout config (issue-33.3 max_flow_duration)
Y el comportamiento observable es consistente
```

### Escenario 4: Bug típico prevenido: nuevo step_type se agrega en 1 lugar

```gherkin
Dado que se agrega un nuevo TargetType "workflow_v2" al sistema
Cuando el dev lo registra
Entonces solo agrega 1 case en el dispatcher (no 3)
Y el dispatcher decide qué runner usar
Y el test e2e que assserta "todos los source ejecutan el nuevo target"
PASA con solo 1 cambio
```

### Escenario 5: Métricas unificadas

```gherkin
Dado que el dispatcher es único
Cuando dispatch ocurre
Entonces se incrementa:
  - metrics.DispatchTotal.WithLabelValues(source, target_type, result).Inc()
Y antes había 3 counters separados (uno por source)
Y la unificación permite queries cross-source ("cuántos flows
corrimos en total hoy?")
```

### Escenario 6: Audit log unificado

```gherkin
Dado que un dispatch ocurre
Cuando se completa (o falla)
Entonces el audit_log tiene:
  {
    actor_user_id: <user si aplica>,
    origin_org_id: <org>,
    action: "dispatch",
    resource: "<target_type>/<target_id>",
    metadata: {source, duration_ms, result: "success"|"failed", error: ...}
  }
Y el campo `source` diferencia el origen (cron/webhook/mcp)
Y queries por "todos los dispatches del día" son triviales
```

### Escenario 7: Sabotaje — dispatcher solo lo usa UNO de los 3

```gherkin
Dado que el refactor solo migra el cron dispatcher (sabotaje)
Y webhook y mcp siguen con su lógica vieja
Cuando se agrega un nuevo step_type
Entonces el cron lo maneja bien, pero webhook y mcp no
Y el test e2e que assserta "los 3 sources ejecutan el nuevo target"
FALLA para webhook y mcp
Cuando restauro la migración completa (los 3 sources usan el dispatcher)
Entonces el test verde
```

### Escenario 8: Edge case — source no reconocido

```gherkin
Dado que un source "experiment_X" llega al dispatcher (no está en la enum)
Cuando dispatch ocurre
Entonces el dispatcher NO crashea
Y registra el evento con source=experiment_X (label dinámico)
Y el log tiene WARNING: "unknown source, dispatching anyway"
Y permite instrumentar nuevos sources sin redeploy del dispatcher
```

## Notas

- El dispatcher es la pieza de "single source of truth" para
  "¿qué hago con este target?". Toda la lógica de selección de
  runner, métricas, audit, timeout, etc. vive acá.
- Es un refactor NO-trivial. Requiere:
  1. Crear el dispatcher.
  2. Migrar los 3 call-sites uno por uno (con tests de paridad).
  3. Eliminar el código viejo (verificar con grep).
  4. Métricas y audit unificados.
- NO cambia el comportamiento observable: el flow se ejecuta
  igual, los errores son los mismos, los timeouts también.
