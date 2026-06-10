# Tasks: issue-15.1-token-tracking

## Backend

- [ ] Crear migración para tabla `token_usage` con índices
- [ ] Implementar modelo `TokenUsage` en Go
- [ ] Implementar `CostCalculator` que usa model registry para precios
- [ ] Implementar `TokenUsageHook` que se integra en LLM Provider Factory
- [ ] Implementar `BatchWriter` con buffer + flush periódico + bulk insert
- [ ] Registrar hook en todos los providers (OpenAI, Anthropic, Ollama, etc.)
- [ ] Pasar RunInfo (run_id, run_type, step_id) en contexto de llamada LLM
- [ ] Implementar store con Create, BulkCreate, y queries de agregación
- [ ] Implementar agregación por: run_id, project_id, model, provider, date_range
- [ ] Exponer endpoints CRUD para token_usage via API factory
- [ ] Implementar TTL policy (auto-delete registros > N días)

## Frontend

- [ ] N/A (API + datos, UI se cubre en issue-15.2)

## Tests

- [ ] Test unitario: CostCalculator con precios mock
- [ ] Test unitario: BatchWriter buffer y flush
- [ ] Test de integración: hook persiste token usage en DB
- [ ] Test de integración: agregación por run
- [ ] Test de integración: agregación por modelo y fecha
- [ ] Test de integración: cost_unknown para modelo no registrado
- [ ] Sabotaje: hook silencioso → test con llamada LLM detecta

## Cierre

- [ ] Verificación manual: ejecutar agente, verificar token_usage en DB
- [ ] Suite verde: `go test ./internal/cost/...`
- [ ] Performance: 1000 llamadas concurrentes, batch writer no pierde datos
