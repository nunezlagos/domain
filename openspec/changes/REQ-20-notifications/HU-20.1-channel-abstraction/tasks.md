# Tasks: HU-20.1-channel-abstraction

## Schema

- [ ] **db-001**: Migración tabla `notification_templates`
- [ ] **db-002**: Migración tabla `notification_subscriptions`
- [ ] **db-003**: Migración tabla `notification_deliveries` con índice partial

## Backend

- [ ] **notif-001**: `internal/notifications/channel.go` interface + registry
- [ ] **notif-002**: `internal/notifications/template.go` strict render
- [ ] **notif-003**: `internal/notifications/service.go` Enqueue(event, payload)
- [ ] **notif-004**: `internal/notifications/worker.go` pool con SKIP LOCKED
- [ ] **notif-005**: `internal/notifications/retry.go` backoff exp
- [ ] **notif-006**: Dead-letter callback notifica admin canal

## Tests

- [ ] **test-001**: Registry concurrent safe
- [ ] **test-002**: Template strict missing key
- [ ] **test-003**: Worker procesa pending
- [ ] **test-004**: Retry transitorio + dead letter
- [ ] **test-005**: Routing por subscription
- [ ] **sabotaje-001**: Loop infinito → max-depth corta

## Docs

- [ ] **docs-001**: `docs/notifications.md` cómo agregar nuevo canal
