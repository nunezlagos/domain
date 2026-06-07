# Tasks: HU-21.4-billing-stripe

- [ ] **st-001**: Dep `stripe-go/v79`
- [ ] **st-002**: Migración subscriptions + stripe_events_processed
- [ ] **st-003**: Service `internal/service/billing.go`
- [ ] **st-004**: Handler checkout-session
- [ ] **st-005**: Handler portal-session
- [ ] **st-006**: Handler webhook con signature verify + idempotency
- [ ] **st-007**: Event handlers (5 events listados en design)
- [ ] **st-008**: Cron reconciliation diario (failsafe missed webhooks)
- [ ] **st-009**: Dunning: notif + downgrade tras 3 fallos
- [ ] **test-001**: Checkout end-to-end con stripe-mock
- [ ] **test-002**: Webhook signature inválida → 401
- [ ] **test-003**: Idempotency double event
- [ ] **test-004**: Dunning flow
- [ ] **docs-001**: `docs/billing.md` setup Stripe keys, products, webhook
