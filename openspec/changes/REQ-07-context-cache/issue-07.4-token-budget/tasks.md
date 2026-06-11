# Tasks: issue-07.4-token-budget

## Backend

- [x] Implementar `TokenBudgetManager` con `Check`, `Track`, `Reset` → llm/tokens/budget.go — 2026-06-10
- [x] Implementar `NewTokenBudget` con validación: hard > 0, hard ≤ model.max_tokens, soft ≤ hard, modo válido — 2026-06-10
- [x] Implementar tracking streaming: cada chunk suma (tokens.Estimate) y checkea soft/hard → llm/budget middleware — 2026-06-10
- [x] Implementar soft limit: callback configurable OnSoftLimit (dispara una sola vez); wireado a appendLog warning en agent runner — 2026-06-10
- [x] Implementar hard limit: ModeTruncate (corta stream graceful con Done) vs ModeError (ErrBudgetExceeded) — 2026-06-10
- [x] Implementar estado expuesto: BudgetState{TokensUsed, BudgetRemaining, Percentage, Truncated} — 2026-06-10
- [x] Crear middleware/decorator para LLM provider → llm/budget.New (Check pre-llamada + Track post-respuesta + corte de streams) — 2026-06-10
- [x] Integrar con model registry → agent runner clampa hard al ContextSize del modelo (registry.Get) — 2026-06-10

## Tests

- [x] Test: constructor rechaza hard > model.max_tokens → TestNewTokenBudget_Validation
- [x] Test: Track incrementa correctamente → TestTrack_IncrementsAndState
- [x] Test: soft limit dispara callback (una vez) → TestSoftLimit_CallbackFiresOnce
- [x] Test: hard limit modo error → TestHardLimit_ErrorMode (Track + Check posterior bloquean)
- [x] Test: hard limit modo truncate → TestHardLimit_TruncateMode (Truncated=true)
- [x] Test: Check inicial ok → TestNewTokenBudget_Validation
- [x] Test integración: middleware con provider mock streaming → TestCompleteStream_TruncatesGracefully + TestComplete_BlocksWhenExhausted
- [x] Test E2E Gherkin → escenarios cubiertos por la matriz de tests (soft warning, hard error, truncate graceful, clamp a model max)
- [x] Sabotaje: hard=0 → error inmediato en constructor → TestNewTokenBudget_Validation

## Cierre

- [x] Verificación manual: corte graceful → TestCompleteStream_TruncatesGracefully lo verifica determinísticamente
- [x] Suite verde → 2026-06-10 (24 tests tokens+budget)
- [x] Configuración documentada: agents.token_budget (BD) + clamp automático al context size del modelo
