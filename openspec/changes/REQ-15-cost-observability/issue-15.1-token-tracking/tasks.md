# Tasks: issue-15.1-token-tracking

## Backend

- [x] Crear migración para tabla `token_usage` con índices
- [x] Implementar modelo `TokenUsage` en Go
- [x] Implementar `CostCalculator` que usa model registry para precios
- [x] Implementar `TokenUsageHook` que se integra en LLM Provider Factory
- [x] Implementar `BatchWriter` con buffer + flush periódico + bulk insert
- [x] Registrar hook en todos los providers (OpenAI, Anthropic, Ollama, etc.)
- [x] Pasar RunInfo (run_id, run_type, step_id) en contexto de llamada LLM
- [x] Implementar store con Create, BulkCreate, y queries de agregación
- [x] Implementar agregación por: run_id, project_id, model, provider, date_range
- [x] Exponer endpoints CRUD para token_usage via API factory
- [x] Implementar TTL policy (auto-delete registros > N días)

## Frontend

- [x] N/A (API + datos, UI se cubre en issue-15.2)

## Tests

- [x] Test unitario: CostCalculator con precios mock
- [x] Test unitario: BatchWriter buffer y flush
- [x] Test de integración: hook persiste token usage en DB
- [x] Test de integración: agregación por run
- [x] Test de integración: agregación por modelo y fecha
- [x] Test de integración: cost_unknown para modelo no registrado
- [x] Sabotaje: hook silencioso → test con llamada LLM detecta

## Cierre

- [x] Verificación manual: ejecutar agente, verificar token_usage en DB
- [x] Suite verde: `go test ./internal/cost/...`
- [x] Performance: 1000 llamadas concurrentes, batch writer no pierde datos
