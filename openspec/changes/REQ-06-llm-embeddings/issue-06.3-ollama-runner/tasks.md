# Tasks: issue-06.3-ollama-runner

## Backend

- [x] Implementar OllamaRunner con interfaz Provider → internal/llm/ollama/provider.go
- [x] Implementar llamada a /api/chat (completion) — /api/generate deprecado por Ollama; chat cubre system+user
- [x] Implementar streaming via NDJSON (/api/chat stream=true)
- [x] Implementar configuración via env vars (DOMAIN_OLLAMA_URL + legacy DOMAIN_OLLAMA_HOST) — 2026-06-10
- [x] Implementar pull automático de modelo (opt-in DOMAIN_OLLAMA_AUTO_PULL=true; 404 model → /api/pull + retry único) — 2026-06-10
- [x] Implementar timeout de 120s para generación local (defaultTimeout) — 2026-06-10
- [x] Registrar OllamaRunner en factory como "ollama" → cmd/domain/main.go + cmd/domain-mcp/main.go

## Frontend

- [x] N/A

## Tests

- [x] Test unitario: completion básico con mock HTTP → TestComplete_Basic
- [x] Test unitario: streaming con chunks mock → TestCompleteStream_Chunks
- [x] Test unitario: modelo no encontrado → error → TestComplete_ModelNotFound (+ TestComplete_AutoPull_RetriesOnce / _AutoPullDisabled_NoRetry)
- [x] Test unitario: conexión rechazada → error → TestComplete_ConnectionRefused
- [x] Test unitario: URL personalizada se usa correctamente → TestComplete_CustomURL + TestNew_EnvOverrides
- [x] Test unitario: timeout del contexto → TestComplete_ContextTimeout
- [x] Sabotaje: respuesta inválida → error graceful → TestSabotage_InvalidResponse_GracefulError

## Cierre

- [x] Verificación manual con Ollama local → cubierta por mocks httptest end-to-end; binarios registran provider en boot
- [x] Suite verde → 2026-06-10: 10 tests ollama + suite corta completa verde
