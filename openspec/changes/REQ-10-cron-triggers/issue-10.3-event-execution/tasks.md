# Tasks: issue-10.3-event-execution

## Backend

- [x] Definir constantes de tipos de evento en `internal/events/types.go`
- [x] Implementar `EventBus` struct con publish/subscribe in-process
- [x] Implementar modelo `EventSubscription` en `internal/models/event.go`
- [x] Implementar modelo `Event` y `EventDelivery`
- [x] Crear migración SQL: tabla `event_subscriptions`
- [x] Crear migración SQL: tabla `events`
- [x] Crear migración SQL: tabla `event_deliveries`
- [x] Implementar `EventSubscriptionRepository` CRUD
- [x] Implementar `EventRepository` (insert event + deliveries)
- [x] Implementar evaluación de filtros (match exacto flat)
- [x] Implementar worker pool para delivery (N goroutines)
- [x] Implementar retry delivery con backoff (5s, 30s, 5min, max 3)
- [x] Implementar detección de profundidad máxima de eventos (5)
- [x] Implementar dead letter para eventos no entregables
- [x] Integrar emisión de eventos en FlowRunner (complete, fail, step_complete, step_fail)
- [x] Integrar emisión de eventos en AgentRunner (complete, fail)
- [x] Integrar emisión de eventos en SkillEngine (complete, fail)
- [x] Integrar emisión de eventos en CronScheduler (executed)
- [x] Integrar emisión de eventos en WebhookReceiver (received)
- [x] Crear handler REST: CRUD /api/v1/event-subscriptions
- [x] Crear handler REST: GET /api/v1/event-subscriptions/:id/deliveries
- [x] Crear handler REST: PATCH /api/v1/event-subscriptions/:id (enable/disable)
- [x] Crear handler REST: GET /api/v1/event-types

## Tests

- [x] Test unitario: bus publish/subscribe single
- [x] Test unitario: bus publish/subscribe multiple
- [x] Test unitario: filtro match exacto
- [x] Test unitario: filtro no match
- [x] Test unitario: filtro vacío (match all)
- [x] Test unitario: retry delivery éxito en intento 2
- [x] Test unitario: retry delivery falla después de 3 intentos
- [x] Test unitario: suscripción deshabilitada no recibe
- [x] Test unitario: profundidad máxima de eventos (5)
- [x] Test de integración: evento flow.completed → subscription ejecuta flow
- [x] Test de integración: 3 suscriptores reciben mismo evento
- [x] Test de integración: delivery history
- [x] Sabotaje: quitar retry → test de retry falla

## Cierre

- [x] Verificación manual: crear suscripción a flow.failed, fallar un flow, ver que el suscriptor se ejecuta
- [x] Verificación manual: delivery history visible
- [x] Suite verde
