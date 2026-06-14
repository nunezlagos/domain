# Tasks: HU-28.7-webhook-goroutine-lifecycle

- [ ] **wh-001**: Implementar worker pool + channel acotado en `handler/webhook.go`
- [ ] **wh-002**: Migrar `dispatchWebhook` a worker pool
- [ ] **wh-003**: Migrar `webhook_admin.go` al mismo pool
- [ ] **wh-004**: Agregar `Shutdown(ctx)` al handler
- [ ] **wh-005**: Integrar shutdown en `cmd/domain/main.go`
- [ ] **wh-006**: Tests: worker procesa job, 503 cuando cola llena, shutdown graceful
- [ ] **wh-007**: Sabotaje: worker bloqueado → shutdown respeta timeout
- [ ] **wh-008**: Suite completa verde
