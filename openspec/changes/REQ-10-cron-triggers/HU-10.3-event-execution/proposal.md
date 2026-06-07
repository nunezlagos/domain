# Proposal: HU-10.3-event-execution

## Intención

Implementar un bus de eventos interno pub/sub donde eventos del sistema (flow.completed, flow.failed, domain_agent_run.completed, etc.) pueden disparar ejecuciones de flows mediante suscripciones configurables con filtros. Entrega at-least-once con retry y delivery history.

## Scope

**Incluye:**
- Modelo `EventSubscription` con: name, project_id, event_type, flow_slug, filter (jsonb), enabled
- CRUD de suscripciones
- Event bus in-process: publicadores emiten eventos, el bus los distribuye a suscriptores
- Lista de tipos de eventos del sistema (definidos como constantes)
- Evaluación de filtros sobre payload del evento
- Delivery at-least-once: retry 3 veces con backoff (5s, 30s, 5min)
- Dead letter para eventos no entregables
- Delivery history (tabla event_deliveries)
- Endpoint GET /api/v1/event-types

**Excluye:**
- Broker externo (Kafka, RabbitMQ) — el bus in-process es suficiente para MVP monolite
- Garantía exactly-once (at-least-once es suficiente; idempotencia del lado del flow)
- Eventos externos (solo eventos internos del sistema)
- Prioridades de eventos

## Enfoque técnico

- Event bus: struct con `map[string][]Subscriber` (event_type → subscribers) y mutex para concurrencia
- Publicación: `bus.Publish(eventType, payload)` que itera subscribers, evalúa filtros, lanza delivery en goroutine
- Suscripción: `bus.Subscribe(eventType, subscriber)` registra un subscriber
- Filtros: evaluación simple de `map[string]interface{}` contra payload.data usando match exacto
  - filter vacío o nil = match all
  - filter `{"flow_slug": "critical"}` = match si payload.data.flow_slug == "critical"
  - Soporte para match simple (valor exacto) y arrays (valor in array)
- Delivery: worker pool (N goroutines) que procesan eventos:
  1. Ejecutar flow asociado
  2. Si éxito → mark delivered
  3. Si fallo → retry con backoff up to 3 veces
  4. Si persiste → mark undelivered
- Puntos de emisión: hooks en flow runner, agent runner, skill engine, cron scheduler, webhook receiver
- Todos los eventos se persisten en tabla `events` para trazabilidad

## Riesgos

- Bucle infinito de eventos: Flow A publica evento → Flow B se ejecuta → publica evento → Flow A... → Detección de ciclos o límite de profundidad (máximo 5 eventos encadenados)
- Sobrecarga del bus: si muchos eventos se publican simultáneamente, el worker pool evita bloqueo
- Filtros complejos: solo soportamos match exacto en primer nivel; filtros anidados no (futuro)

## Testing

- Unit: bus publish/subscribe
- Unit: evaluación de filtros (match, no match, vacío, array)
- Unit: retry delivery (success en intento 2, failure after 3)
- Unit: suscripción deshabilitada no recibe eventos
- Integration: flujo completo: evento publicado → flow ejecutado
- Integration: múltiples suscriptores al mismo evento
- Sabotaje: quitar retry → test de entrega fallida sin retry
