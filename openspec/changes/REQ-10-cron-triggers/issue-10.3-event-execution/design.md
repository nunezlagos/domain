# Design: issue-10.3-event-execution

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Bus de eventos | In-process con channels + mutex | RabbitMQ, NATS, Kafka (innecesario para MVP monolite) |
| Evaluación de filtros | Match exacto de map flat | JSONPath, expr (match exacto cubre 80% de casos, es simple y rápido) |
| Delivery worker | Pool de goroutines (buffered channel) | Goroutine por evento (pool evita unbounded growth) |
| Persistencia | Tablas Postgres (events, event_deliveries) | Solo en memoria (necesitamos trazabilidad y recovery) |
| Prevención de ciclos | Límite de profundidad (5) + event_id trace | Graph global (límite simple es suficiente para MVP) |

## Alternativas descartadas

- **Broker externo (RabbitMQ/Kafka)**: Potente pero agrega complejidad operativa. Para MVP, un bus in-process con channels es suficiente. Si el sistema crece horizontalmente (múltiples réplicas), migraríamos a un broker.
- **Exactly-once delivery**: Requiere deduplication con IDs de evento + almacenamiento de processed_ids. At-least-once + idempotencia en flows es más simple y práctico.
- **Evaluación de filtros con expr/JSONPath**: Más potente pero más caro computacionalmente y más complejo. Match exacto de primer nivel cubre el caso de uso común: "ejecutar flow X solo cuando Y = Z".

## Diagrama

```
                    ┌─────────────┐
                    │  Emisores   │
                    └──────┬──────┘
                           │
          ┌────────────────┼──────────────────┐
          │                │                  │
          ▼                ▼                  ▼
┌────────────────┐ ┌──────────────┐ ┌────────────────┐
│ Flow Runner    │ │ Agent Runner │ │ Cron/Webhook   │
│ (on complete)  │ │ (on fail)    │ │ (on execute)   │
│ (on fail)      │ │ (on complete)│ │                │
└───────┬────────┘ └──────┬───────┘ └───────┬────────┘
        │                 │                 │
        │     bus.Publish(eventType, data)  │
        └─────────────────┼─────────────────┘
                          │
                          ▼
                 ┌─────────────────┐
                 │   Event Bus     │
                 │                 │
                 │ map[eventType]  │
                 │   → []Subscriber│
                 └────────┬────────┘
                          │
                    ┌─────┴─────┐
                    │ Eval filtro│
                    └─────┬─────┘
                          │
                    ┌─────┴─────┐
                    │ Worker    │
                    │ Pool (N)  │
                    └─────┬─────┘
                          │
                    ┌─────┴─────┐
                    │ Ejecutar  │
                    │ flow      │
                    └─────┬─────┘
                          │
                    ┌─────┴─────┐
                    │ Retry?    │── sí ──► esperar backoff → loop
                    └─────┬─────┘
                          │ no
                    ┌─────┴─────┐
                    │ Log entrega│
                    └───────────┘
```

Modelos:
```sql
CREATE TABLE event_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id),
    event_type VARCHAR(100) NOT NULL,
    flow_slug VARCHAR(255) NOT NULL,
    filter JSONB DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_event_subs_type ON event_subscriptions(event_type, enabled);

CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    source VARCHAR(100) NOT NULL,
    data JSONB NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE event_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id),
    subscription_id UUID NOT NULL REFERENCES event_subscriptions(id),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    flow_run_id UUID REFERENCES flow_runs(id),
    attempt_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Tipos de eventos (constantes):
```go
const (
    EventFlowCompleted    = "flow.completed"
    EventFlowFailed       = "flow.failed"
    EventFlowStepCompleted = "flow.step_completed"
    EventFlowStepFailed   = "flow.step_failed"
    EventAgentRunCompleted = "domain_agent_run.completed"
    EventAgentRunFailed   = "domain_agent_run.failed"
    EventSkillCompleted   = "skill.completed"
    EventSkillFailed      = "skill.failed"
    EventWebhookReceived  = "webhook.received"
    EventCronExecuted     = "cron.executed"
)
```

## TDD plan

1. **Red:** Test `TestEventBus_PublishSubscribe` — subscriber recibe evento
2. **Green:** Implementar bus con map[type][]subscriber
3. **Red:** Test `TestEventBus_MultipleSubscribers` — todos reciben
4. **Green:** Iterar todos los subscribers
5. **Red:** Test `TestFilter_EvaluateMatch` — filter match exacto
6. **Green:** Evaluar filter contra payload.data
7. **Red:** Test `TestFilter_NoMatch` — no ejecuta
8. **Green:** Skip si filter no match
9. **Red:** Test `TestFilter_Empty` — match all
10. **Green:** filter vacío = todos
11. **Red:** Test `TestDelivery_RetrySuccess` — success en intento 2
12. **Green:** Retry worker con backoff
13. **Red:** Test `TestDelivery_RetryFailure` — undelivered after 3 attempts
14. **Green:** Marcar undelivered + log
15. **Red:** Test `TestSubscription_Disabled` — skip if disabled
16. **Green:** Verificar enabled antes de deliver
17. **Red:** Test `TestEventDepthLimit` — 5 eventos encadenados → 6to no
18. **Green:** Contador de profundidad en contexto de evento
19. **Sabotaje:** Sacar retry → test de retry falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Bucle infinito eventos | Baja | Alto | Límite de profundidad 5 + event_id trace |
| Pérdida de eventos si bus crashea | Media | Alto | Persistencia en tabla events ANTES de publicar |
| Sobrecarga de ejecuciones | Media | Medio | Worker pool limitado (default 10 goroutines) |
| Filtro demasiado amplio | Media | Bajo | Match exacto solo; documentar que no soporta regex |
