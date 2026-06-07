# Tasks: HU-03.3-prompts-storage

## Backend

- [ ] `migrations/XXXX_create_prompts.sql`: tabla + índice GIN tsvector + FK a sessions
- [ ] `internal/store/pg/prompt.go`: interfaz `PromptStore` + structs
- [ ] Implementar `Insert(p Prompt) (uuid.UUID, error)`
- [ ] Implementar `BatchInsert(prompts []Prompt) error`
- [ ] Implementar `Search(query string, filter PromptFilter) ([]PromptSearchResult, int, error)`
- [ ] Implementar `ListBySession(sessionID string, limit, offset int) ([]Prompt, int, error)`
- [ ] Implementar `Delete(id uuid.UUID) error`
- [ ] `internal/memory/buffer.go`: `PromptBuffer` struct con channel, ticker, batch, done, wg
- [ ] Implementar worker con select multipiso (ch, ticker, done)
- [ ] Implementar `flush()` con batch insert
- [ ] Implementar `Shutdown()` con flush final + wait
- [ ] `internal/memory/service.go`: integrar `SavePrompt` con buffer
- [ ] Config: buffer size, flush interval, batch size via config system

## Tests

- [ ] Test unitario de buffer: insertar N prompts → verificar flush
- [ ] Test de buffer lleno: verificar que descarta con log (no panic)
- [ ] Test de graceful shutdown: insertar → shutdown → todos persistidos
- [ ] Test de integración: insert → search → verify tsvector rank
- [ ] Test de paginación con ListBySession
- [ ] Sabotaje: matar worker abruptamente → verificar que prompts en buffer se pierden (comportamiento esperado)

## Cierre

- [ ] Verificación manual: guardar prompt, buscar por palabra clave
- [ ] Suite verde
