# Design: issue-07.4-token-budget

## Decisión arquitectónica

**Patrón:** Decorator/Middleware sobre el LLM provider.

```
LLMProvider
    │
    ▼
TokenBudgetMiddleware ──▶ check budget antes de cada call streaming
    │                        │
    ▼                        ▼
LLMProvider real ──────▶ track tokens por chunk
    │                        │
    ▼                        ▼
Response               check hard/soft en cada chunk
```

## Alternativas descartadas

1. **Budget en el runner (REQ-11):** El runner no debería conocer detalles de token counting. Mejor como middleware separado.
2. **Budget post-hoc (medir al final):** No permite corte durante streaming. Necesitamos tracking en vivo.
3. **Budget global compartido:** Complejidad innecesaria para MVP. Budget por run es suficiente.

## Diagrama

```
┌─────────────────────────────────────────────────────┐
│ TokenBudgetManager                                   │
│                                                      │
│  BudgetConfig {                                       │
│    HardLimit: 4096,                                   │
│    SoftLimit: 3072,                                   │
│    Mode: "truncate" | "error"                         │
│    ModelMaxTokens: 8192                               │
│  }                                                   │
│                                                      │
│  State {                                              │
│    TokensUsed: 2000,                                  │
│    SoftReached: false,                                │
│    HardReached: false,                                │
│    Truncated: false                                    │
│  }                                                   │
│                                                      │
│  Methods {                                            │
│    Check(n) → (ok, warning)                           │
│    Track(n) → (consumption, remaining, pct)           │
│    Reset()                                            │
│  }                                                   │
└─────────────────────────────────────────────────────┘
```

## TDD plan

1. **Red:** Test que `NewTokenBudget(agent, model)` rechaza hard_limit > model.max_tokens
2. **Green:** Implementar constructor con validación
3. **Refactor:** Agregar tracking y callbacks
4. **Sabotaje:** Hard limit = 0 → cualquier Track() lanza error inmediato

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Token counting streaming impreciso | Usar usage.total_tokens del provider al final para corrección |
| Soft limit callback bloquea | Ejecutar callback en goroutine con select non-blocking |
| Budget mal configurado (hard < soft) | Validar en constructor: hard >= soft siempre |
| Model registry no tiene max_tokens para ese modelo | Default a 4096 y loggear warning
