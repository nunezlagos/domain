# Tasks: issue-10.3-event-execution

## Backend

- [ ] Definir constantes de tipos de evento en `internal/events/types.go`
- [ ] Implementar `EventBus` struct con publish/subscribe in-process
- [ ] Implementar modelo `EventSubscription` en `internal/models/event.go`
- [ ] Implementar modelo `Event` y `EventDelivery`
- [ ] Crear migración SQL: tabla `event_subscriptions`
- [ ] Crear migración SQL: tabla `events`
- [ ] Crear migración SQL: tabla `event_deliveries`
- [ ] Implementar `EventSubscriptionRepository` CRUD
- [ ] Implementar `EventRepository` (insert event + deliveries)
- [ ] Implementar evaluación de filtros (match exacto flat)
- [ ] Implementar worker pool para delivery (N goroutines)
- [ ] Implementar retry delivery con backoff (5s, 30s, 5min, max 3)
- [ ] Implementar detección de profundidad máxima de eventos (5)
- [ ] Implementar dead letter para eventos no entregables
- [ ] Integrar emisión de eventos en FlowRunner (complete, fail, step_complete, step_fail)
- [ ] Integrar emisión de eventos en AgentRunner (complete, fail)
- [ ] Integrar emisión de eventos en SkillEngine (complete, fail)
- [ ] Integrar emisión de eventos en CronScheduler (executed)
- [ ] Integrar emisión de eventos en WebhookReceiver (received)
- [ ] Crear handler REST: CRUD /api/v1/event-subscriptions
- [ ] Crear handler REST: GET /api/v1/event-subscriptions/:id/deliveries
- [ ] Crear handler REST: PATCH /api/v1/event-subscriptions/:id (enable/disable)
- [ ] Crear handler REST: GET /api/v1/event-types

## Tests

- [ ] Test unitario: bus publish/subscribe single
- [ ] Test unitario: bus publish/subscribe multiple
- [ ] Test unitario: filtro match exacto
- [ ] Test unitario: filtro no match
- [ ] Test unitario: filtro vacío (match all)
- [ ] Test unitario: retry delivery éxito en intento 2
- [ ] Test unitario: retry delivery falla después de 3 intentos
- [ ] Test unitario: suscripción deshabilitada no recibe
- [ ] Test unitario: profundidad máxima de eventos (5)
- [ ] Test de integración: evento flow.completed → subscription ejecuta flow
- [ ] Test de integración: 3 suscriptores reciben mismo evento
- [ ] Test de integración: delivery history
- [ ] Sabotaje: quitar retry → test de retry falla

## Cierre

- [ ] Verificación manual: crear suscripción a flow.failed, fallar un flow, ver que el suscriptor se ejecuta
- [ ] Verificación manual: delivery history visible
- [ ] Suite verde
