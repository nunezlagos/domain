# Tasks: HU-07.4-token-budget

## Backend

- [ ] Implementar `TokenBudgetManager` struct con `Check`, `Track`, `Reset` métodos
- [ ] Implementar `NewTokenBudget(agent, model)` con validación: hard ≤ model.max_tokens, hard ≥ soft
- [ ] Implementar tracking streaming: cada chunk suma a TokensUsed, checkea soft/hard
- [ ] Implementar soft limit: callback configurable (log + warning event)
- [ ] Implementar hard limit: modo "truncate" (corta streaming graceful) vs "error" (lanza error)
- [ ] Implementar estado expuesto: TokensUsed, BudgetRemaining, Percentage, Truncated
- [ ] Crear middleware/decorator para LLM provider que envuelva calls con budget checking
- [ ] Integrar con model registry (HU-06.4) para obtener max_tokens por modelo

## Tests

- [ ] Test unitario: constructor rechaza hard > model.max_tokens
- [ ] Test unitario: Track incrementa TokensUsed correctamente
- [ ] Test unitario: soft limit dispara callback
- [ ] Test unitario: hard limit en modo "error" lanza ErrBudgetExceeded
- [ ] Test unitario: hard limit en modo "truncate" corta y marca Truncated=true
- [ ] Test unitario: Check antes de llamada inicial retorna ok
- [ ] Test de integración: middleware wrapping LLM provider mock con streaming
- [ ] Test E2E: escenarios Gherkin del hu.md
- [ ] Sabotaje: hard=0 → Track() lanza error inmediato

## Cierre

- [ ] Verificación manual: run con hard limit bajo → confirmar corte graceful
- [ ] Suite verde completa
- [ ] Documentar configuración de budgets por agente/flow
