# Tasks: issue-03.3-prompts-storage

## Backend

- [x] `migrations/XXXX_create_prompts.sql`: tabla + índice GIN tsvector + FK a sessions
- [x] `internal/store/pg/prompt.go`: interfaz `PromptStore` + structs
- [x] Implementar `Insert(p Prompt) (uuid.UUID, error)`
- [x] Implementar `BatchInsert(prompts []Prompt) error`
- [x] Implementar `Search(query string, filter PromptFilter) ([]PromptSearchResult, int, error)`
- [x] Implementar `ListBySession(sessionID string, limit, offset int) ([]Prompt, int, error)`
- [x] Implementar `Delete(id uuid.UUID) error`
- [x] `internal/memory/buffer.go`: `PromptBuffer` struct con channel, ticker, batch, done, wg
- [x] Implementar worker con select multipiso (ch, ticker, done)
- [x] Implementar `flush()` con batch insert
- [x] Implementar `Shutdown()` con flush final + wait
- [x] `internal/memory/service.go`: integrar `SavePrompt` con buffer
- [x] Config: buffer size, flush interval, batch size via config system

## Tests

- [x] Test unitario de buffer: insertar N prompts → verificar flush
- [x] Test de buffer lleno: verificar que descarta con log (no panic)
- [x] Test de graceful shutdown: insertar → shutdown → todos persistidos
- [x] Test de integración: insert → search → verify tsvector rank
- [x] Test de paginación con ListBySession
- [x] Sabotaje: matar worker abruptamente → verificar que prompts en buffer se pierden (comportamiento esperado)

## Cierre

- [x] Verificación manual: guardar prompt, buscar por palabra clave
- [x] Suite verde
