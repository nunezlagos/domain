# issue-09.3-flow-state-machine

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** ingeniero de automatización
**Quiero** que los flows se ejecuten con una state machine que transicione eventos (step_completed → trigger next) y mantenga estado por step y por flow
**Para** tener visibilidad total del progreso, poder pausar/reanudar flujos y debuggear ejecuciones paso a paso

## Criterios de aceptación

### Escenario 1: Flow ejecuta secuencia lineal de principio a fin

```gherkin
Dado que existe un flow definido con steps [s1, s2, s3] en secuencia lineal (s1→s2→s3)
Cuando ejecuto el flow
Entonces el flow pasa a estado `running`
Y el step s1 pasa a estado `step_running`
Cuando s1 completa exitosamente
Entonces s1 pasa a estado `step_completed`
Y automáticamente s2 pasa a estado `step_running`
Cuando s2 completa exitosamente
Entonces s2 pasa a estado `step_completed`
Y automáticamente s3 pasa a estado `step_running`
Cuando s3 completa exitosamente
Entonces s3 pasa a estado `step_completed`
Y el flow pasa a estado `completed`
```

### Escenario 2: Flow falla cuando un step falla

```gherkin
Dado que existe un flow con steps [s1, s2, s3]
Cuando s1 se ejecuta exitosamente
Y s2 falla con un error
Entonces s2 pasa a estado `step_failed`
Y el flow pasa a estado `failed`
Y s3 nunca se ejecuta
Y el resultado del flow contiene `failed_step: "s2"`
Y el resultado contiene el mensaje de error de s2
```

### Escenario 3: Flow se puede pausar y reanudar

```gherkin
Dado que un flow está en estado `running` con s1 en estado `step_running`
Cuando envío un POST a `/api/v1/flow-runs/{run_id}/pause`
Entonces el flow pasa a estado `paused`
Y el step actual (s1) se interrumpe (context cancelado)
Y el estado se persiste para reanudación

Cuando envío un POST a `/api/v1/flow-runs/{run_id}/resume`
Entonces el flow pasa a estado `running`
Y el step s1 se reintenta desde el inicio
Y la ejecución continúa normalmente
```

### Escenario 4: Flow se puede cancelar

```gherkin
Dado que un flow está en estado `running`
Cuando envío un POST a `/api/v1/flow-runs/{run_id}/cancel`
Entonces el flow pasa a estado `cancelled`
Y todos los steps en ejecución se cancelan (context cancelado)
Y no se ejecutan más steps
Y el resultado del flow contiene `cancelled_at` con timestamp
```

### Escenario 5: State machine transiciona correctamente todos los estados

```gherkin
Dado un flow run con estado inicial `pending`
Cuando se inicia la ejecución
Entonces el estado transiciona a `running`

Dado un flow run en estado `running`
Cuando el último step se completa
Entonces el estado transiciona a `completed`

Dado un flow run en estado `running`
Cuando un step falla
Entonces el estado transiciona a `failed`

Dado un flow run en estado `running`
Cuando recibe pause
Entonces el estado transiciona a `paused`

Dado un flow run en estado `paused`
Cuando recibe resume
Entonces el estado transiciona a `running`

Dado un flow run en estado `running`
Cuando recibe cancel
Entonces el estado transiciona a `cancelled`

Dado un flow run en estado `paused`
Cuando recibe cancel
Entonces el estado transiciona a `cancelled`

Dado un flow run en estado `completed`
Cuando recibe cancel
Entonces la transición es rechazada con error
```

### Escenario 6: Event-driven transitions entre steps

```gherkin
Dado un flow con DAG: s1 → [s2a, s2b] → s3 (s1 completa, luego s2a y s2b en paralelo, luego s3)
Cuando s1 completa exitosamente
Entonces el sistema emite el evento `step_completed` con data de s1
Y el state machine detecta que s2a y s2b están listos (dependencias satisfechas)
Y ambos pasan a `step_running` simultáneamente

Cuando s2a y s2b completan exitosamente
Entonces el sistema emite eventos `step_completed` para cada uno
Y el state machine detecta que s3 tiene todas sus dependencias
Y s3 pasa a `step_running`

Cuando s3 completa
Entonces el flow pasa a `completed`
```

### Escenario 7: Context passing entre steps

```gherkin
Dado un flow con steps [s1, s2]
Cuando s1 completa con resultado `{"email": "test@example.com", "score": 95}`
Y s2 comienza a ejecutarse
Entonces s2 tiene acceso a `steps.s1.result` con los valores de s1
Y s2 puede usar esos valores en su template `{{steps.s1.result.email}}`

Dado que un step anterior falló
Entonces el resultado NO está disponible en el contexto (campo vacío)
Y el step dependiente puede detectar que la dependencia falló
```

### Escenario 8: Consultar estado de ejecución

```gherkin
Dado que existe un flow run con id "run-abc"
Cuando envío un GET a `/api/v1/flow-runs/run-abc`
Entonces el body contiene:
  """
  {
    "id": "run-abc",
    "flow_id": "...",
    "status": "running",
    "current_step_id": "s2",
    "steps": {
      "s1": {"status": "step_completed", "result": {...}, "started_at": "...", "completed_at": "..."},
      "s2": {"status": "step_running", "started_at": "..."},
      "s3": {"status": "step_pending"}
    },
    "started_at": "...",
    "error": null
  }
  """
```

## Análisis breve

- **Qué pide realmente:** State machine determinística para ejecución de flows con transiciones event-driven, contexto compartido entre steps, y API de consulta de estado en tiempo real.
- **Módulos sospechados:** `internal/flow/state_machine.go`, `internal/flow/runner.go`, `internal/models/domain_flow_run.go`, `internal/api/handlers/domain_flow_run.go`
- **Riesgos / dependencias:** Condiciones de carrera en transiciones concurrentes (parallel steps). Persistencia de estado para pause/resume.
- **Esfuerzo tentativo:** L

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
