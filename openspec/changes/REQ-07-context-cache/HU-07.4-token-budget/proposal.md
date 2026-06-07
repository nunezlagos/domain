# Proposal: HU-07.4-token-budget

## Intención

Crear un `TokenBudgetManager` que imponga límites configurados (hard/soft) por agente/flow, valide contra el límite del modelo desde el registry, trackee consumo durante streaming, y maneje agotamiento con truncamiento graceful o error según configuración.

## Scope

**In scope:**
- Módulo `TokenBudgetManager` con métodos `Check(ctx, tokens)`, `Track(ctx, delta)`, `Reset()`
- Hard limit: interrupción forzosa al alcanzarlo
- Soft limit: warning/configurable callback al alcanzarlo
- Validación contra model registry: hard_limit ≤ model.max_tokens
- Tracking streaming con callback de progreso (tokens_usados, budget_restante, %)
- Modo exhaust: `truncate` (cortar graceful) vs `error` (lanzar excepción)

**Out of scope:**
- Budgets compartidos entre múltiples agentes (global pool)
- Budgets basados en costo USD (solo tokens)

## Enfoque técnico

- `TokenBudget` struct por run: `{HardLimit, SoftLimit, TokensUsed, Mode, OnSoftLimit func()}`
- `NewTokenBudget(agent, model)` constructor que valida límites
- Durante streaming: cada chunk recibido → `Track(len(chunk_tokens))` → check soft/hard
- Si soft alcanzado: llama callback (log + warning event)
- Si hard alcanzado: corta stream, setea `Truncated=true`, devuelve partial output + error event
- Modo `error`: hard limit lanza `ErrBudgetExceeded`
- Modo `truncate`: hard limit corta y devuelve output parcial con flag

## Riesgos

- Token counting en streaming (por chunk) puede ser impreciso si los chunks son parciales → usar token counter del provider que devuelve uso_total al final
- Race condition si múltiples streams comparten el mismo budget → budget es por run, no compartido
- Soft limit callback no debe bloquear → ejecutar en goroutine separada

## Testing

- **Unit:** Budget constructor validation, soft/hard limit checks, track increments
- **Integration:** Budget manager + streaming LLM mock
- **Gherkin:** Escenarios del hu.md
- **Sabotaje:** Hard limit = 0 → verificar que cualquier token lanza error inmediato
