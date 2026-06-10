# issue-10.3-event-execution

**Origen:** `REQ-10-cron-triggers`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** ingeniero de automatización
**Quiero** que eventos internos de la plataforma (skill_completed, agent_run_done, flow_failed, etc.) puedan disparar otros flows automáticamente mediante un bus pub/sub con entrega at-least-once
**Para** crear pipelines reactivos donde la finalización de un proceso inicie otro sin intervención manual

## Criterios de aceptación

### Escenario 1: Suscribir un flow a un evento interno

```gherkin
Dado que soy un usuario autenticado
Cuando envío un POST a `/api/v1/event-subscriptions` con:
  """
  {
    "name": "On Flow Failed -> Notify",
    "project_id": "proj-abc-123",
    "event_type": "flow.failed",
    "flow_slug": "send-failure-notification",
    "filter": {"flow_slug": "critical-pipeline"},
    "enabled": true
  }
  """
Entonces el sistema responde con HTTP 201
Y el body contiene el id de la suscripción
```

### Escenario 2: Evento publicado → suscriptores reciben

```gherkin
Dado que existe una suscripción para evento `flow.failed` que ejecuta el flow "send-failure-notification"
Cuando un flow falla (evento `flow.failed` es publicado)
Entonces el bus de eventos recibe el evento con payload:
  """
  {
    "type": "flow.failed",
    "timestamp": "2026-06-07T12:00:00Z",
    "data": {
      "flow_run_id": "run-abc",
      "flow_slug": "critical-pipeline",
      "error": "timeout exceeded",
      "project_id": "proj-abc-123"
    }
  }
  """
Y el bus encuentra todos los subscribers para `flow.failed`
Y evalúa los filtros de cada suscriptor
Y para el suscriptor con filter `{"flow_slug": "critical-pipeline"}`, el filtro hace match
Y ejecuta el flow "send-failure-notification" con el payload del evento como input
Y registra una entrega exitosa
```

### Escenario 3: Filtro no hace match → no ejecuta

```gherkin
Dado que existe una suscripción con filter `{"flow_slug": "critical-pipeline"}`
Cuando se publica un evento `flow.failed` con `data.flow_slug: "other-pipeline"`
Entonces el filtro NO hace match
Y no se ejecuta ningún flow
Y se registra un log indicando "filter_no_match"
```

### Escenario 4: Múltiples suscriptores al mismo evento

```gherkin
Dado que existen 3 suscripciones para `flow.failed`:
  - notify-admins (flow_slug: "send-failure-notification", filter: {})
  - log-to-db (flow_slug: "log-failure", filter: {"flow_slug": "critical-pipeline"})
  - restart-pipeline (flow_slug: "restart-pipeline", filter: {"flow_slug": "critical-pipeline"})
Cuando se publica un evento `flow.failed` con `data.flow_slug: "critical-pipeline"`
Entonces los 3 suscriptores hacen match (filter vacío = todos los eventos)
Y se ejecutan los 3 flows
Y cada ejecución se registra independientemente
```

### Escenario 5: Tipos de eventos soportados

```gherkin
Dado que el sistema soporta los siguientes tipos de eventos:
  | event_type               | descripción                              |
  |--------------------------|------------------------------------------|
  | flow.completed           | Flow completado exitosamente             |
  | flow.failed              | Flow falló                               |
  | flow.step_completed      | Step individual completado               |
  | flow.step_failed         | Step individual falló                    |
  | domain_agent_run.completed      | Agente completó su ejecución             |
  | domain_agent_run.failed         | Agente falló                             |
  | skill.completed          | Skill ejecutado exitosamente             |
  | skill.failed             | Skill falló                              |
  | webhook.received         | Webhook recibido (post-ejecución)        |
  | cron.executed            | Cron schedule ejecutado                  |
Cuando consulto GET /api/v1/event-types
Entonces recibo la lista completa de tipos de eventos soportados
```

### Escenario 6: Entrega at-least-once con retry

```gherkin
Dado que existe una suscripción para `flow.completed`
Y el flow destino está caído (no responde)
Cuando se publica un evento
Entonces el bus intenta entregar el evento
Y si falla, reintenta hasta 3 veces con backoff de 5s, 30s, 5min
Y si después de 3 intentos sigue fallando, el evento se marca como `undelivered`
Y queda en una cola de dead letters para inspección manual

Dado que el segundo reintento tiene éxito
Entonces el evento se marca como `delivered`
Y no se realizan más reintentos
```

### Escenario 7: Historial de entregas

```gherkin
Dado que existen 20 eventos publicados
Cuando envío un GET a `/api/v1/event-subscriptions/{id}/deliveries?limit=5`
Entonces recibo las últimas 5 entregas con:
  """
  {
    "data": [
      {
        "event_id": "evt-001",
        "event_type": "flow.completed",
        "status": "delivered",
        "flow_run_id": "run-xyz",
        "attempts": 1,
        "delivered_at": "..."
      },
      {
        "event_id": "evt-002",
        "event_type": "flow.failed",
        "status": "undelivered",
        "error": "flow not found",
        "attempts": 3,
        "delivered_at": null
      }
    ],
    "pagination": {"total": 20, "limit": 5, "offset": 0}
  }
  """
```

### Escenario 8: Habilitar/deshabilitar suscripción

```gherkin
Dado que una suscripción está deshabilitada (`enabled: false`)
Cuando se publica un evento del tipo suscrito
Entonces el bus ignora la suscripción deshabilitada
Y no ejecuta el flow asociado

Cuando habilito la suscripción via PATCH
Entonces las próximas publicaciones ejecutan el flow normalmente
```

## Análisis breve

- **Qué pide realmente:** Bus de eventos interno pub/sub con suscripciones configurables, filtros por payload, entrega at-least-once con retry, y delivery history.
- **Módulos sospechados:** `internal/events/bus.go`, `internal/events/subscription.go`, `internal/events/delivery.go`, `internal/api/handlers/events.go`
- **Riesgos / dependencias:** El bus debe ser in-process (no external broker para MVP). Depende de REQ-09 (flow execution) y REQ-08 (agent execution).
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
