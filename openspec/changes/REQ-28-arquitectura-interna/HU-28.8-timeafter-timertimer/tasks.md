# Tasks: HU-28.8-timeafter-timertimer

- [ ] **ta-001**: Migrar `llm/retry/retry.go:80` — time.After → time.NewTimer + defer Stop
- [ ] **ta-002**: Migrar `llm/retry/retry.go:107` — idem
- [ ] **ta-003**: Migrar `mcp/server/resilience.go:200` — idem (cuidado con select múltiple)
- [ ] **ta-004**: Tests: verificar que cancelación temprana no deja timers colgados (opcional, difícil de testear deterministicamente)
- [ ] **ta-005**: Suite completa verde
